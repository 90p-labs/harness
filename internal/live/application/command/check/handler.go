package check

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/live/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	spanLiveCheck    = "live.check"
	metricLiveChecks = "harness.live.checks"
	runErrorFormat   = "run %s: %w"
)

const (
	attrRunID    = attribute.Key("harness.run.id")
	attrWorktree = attribute.Key("harness.worktree")
	attrPhase    = attribute.Key("harness.phase")
	attrOutcome  = attribute.Key("harness.live.outcome")
)

type Command struct {
	Repo string
	Now  time.Time
}

type Result struct {
	RunID   string
	Phase   domain.Phase
	Outcome domain.Outcome
	Err     error
}

type Handler struct {
	runs        domain.Runs
	releases    domain.ReleaseRepository
	github      domain.GitHub
	folder      domain.RunFolder
	bus         shareddomain.Dispatcher
	environment string
	rollback    string
	tracer      trace.Tracer
	checks      metric.Int64Counter
}

func NewHandler(
	runs domain.Runs,
	releases domain.ReleaseRepository,
	github domain.GitHub,
	folder domain.RunFolder,
	bus shareddomain.Dispatcher,
	environment, rollback string,
	tracer trace.Tracer,
	meter metric.Meter,
) Handler {
	checks, _ := meter.Int64Counter(metricLiveChecks)
	return Handler{
		runs: runs, releases: releases, github: github, folder: folder, bus: bus,
		environment: environment, rollback: rollback, tracer: tracer, checks: checks,
	}
}

func (h Handler) Handle(ctx context.Context, cmd Command) ([]Result, error) {
	runs, err := h.runs.Live(ctx, cmd.Repo)
	if err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(runs))
	var errs []error
	for _, run := range runs {
		result := h.checkRun(ctx, run, cmd.Now)
		if result.Err != nil {
			errs = append(errs, result.Err)
		}
		results = append(results, result)
	}
	return results, errors.Join(errs...)
}

func (h Handler) checkRun(ctx context.Context, run domain.LiveRun, now time.Time) Result {
	ctx, span := h.tracer.Start(ctx, spanLiveCheck, trace.WithAttributes(
		attrRunID.String(run.ID), attrWorktree.String(run.Worktree), attrPhase.String(string(run.Phase)),
	))
	defer span.End()

	var outcome domain.Outcome
	var err error
	if run.Phase == domain.PhaseReleasing {
		outcome, err = h.checkReleasing(ctx, run, now)
	} else {
		outcome, err = h.checkObserving(ctx, run, now)
	}
	switch {
	case errors.Is(err, domain.ErrStaleRun):
		outcome, err = domain.OutcomeSkipped, nil
	case err != nil:
		outcome, err = domain.OutcomeError, fmt.Errorf(runErrorFormat, run.ID, err)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	span.SetAttributes(attrOutcome.String(string(outcome)))
	h.checks.Add(ctx, 1, metric.WithAttributes(attrOutcome.String(string(outcome))))
	return Result{RunID: run.ID, Phase: run.Phase, Outcome: outcome, Err: err}
}

func (h Handler) checkReleasing(ctx context.Context, run domain.LiveRun, now time.Time) (domain.Outcome, error) {
	release, err := h.releases.Get(ctx, run.ID)
	if errors.Is(err, domain.ErrReleaseNotFound) || (err == nil && release.PRURL != run.PRURL) {
		release, err = domain.Release{RunID: run.ID, PRURL: run.PRURL, Environment: h.environment}, nil
	}
	if err != nil {
		return "", err
	}
	merge, err := h.github.Merge(ctx, run.PRURL)
	if err != nil {
		return "", err
	}
	facts := domain.Facts{Merge: merge}
	merged := merge.State == domain.MergeMerged
	if merged {
		release.MergeSHA, release.MergedAt = merge.SHA, merge.At
		if facts.Deployment, err = h.github.Deployment(ctx, run.PRURL, merge.SHA, h.environment); err != nil {
			return "", err
		}
	}
	decision := domain.Decide(domain.PhaseReleasing, facts, now)
	if decision.Event == domain.EventDeploy {
		release.DeployedAt = facts.Deployment.At
	}
	if merged {
		if err := h.releases.Save(ctx, release); err != nil {
			return "", err
		}
	}
	return h.finish(ctx, run, release, decision, nil)
}

func (h Handler) checkObserving(ctx context.Context, run domain.LiveRun, now time.Time) (domain.Outcome, error) {
	release, err := h.releases.Get(ctx, run.ID)
	if err != nil {
		return "", err
	}
	thresholds, err := h.runs.Signals(ctx, run)
	if err != nil {
		unread := domain.Facts{DeployedAt: release.DeployedAt, Failures: 1}
		return h.finish(ctx, run, release, domain.Decide(domain.PhaseObserving, unread, now), err)
	}
	facts := domain.Facts{DeployedAt: release.DeployedAt, Window: domain.WatchWindow(thresholds)}
	for _, threshold := range thresholds {
		dispatched, err := h.bus.Dispatch(ctx, shareddomain.Envelope{
			Version: shareddomain.EnvelopeVersion,
			Event:   shareddomain.EventSignalsCheck,
			Cwd:     run.Worktree,
			Run:     shareddomain.RunRef{ID: run.ID, Phase: string(run.Phase)},
			Payload: signalPayload(domain.NewSignalCheck(threshold, release, now)),
		})
		if err != nil {
			return "", err
		}
		facts.Breaches = append(facts.Breaches, breaches(threshold, dispatched.Verdict)...)
		facts.Failures += len(dispatched.Failures)
	}
	return h.finish(ctx, run, release, domain.Decide(domain.PhaseObserving, facts, now), nil)
}

func (h Handler) finish(ctx context.Context, run domain.LiveRun, release domain.Release, decision domain.Decision, signalsErr error) (domain.Outcome, error) {
	var reason string
	round := run.LiveRound + 1
	switch decision.Event {
	case domain.EventNone:
		return decision.Outcome, nil
	case domain.EventRegress:
		reason = domain.Findings{
			Release: release, RunID: run.ID, Round: round,
			Breaches: decision.Breaches, DeployFailure: decision.DeployFailure, Rollback: h.rollback,
		}.Text()
	case domain.EventBlock:
		reason = domain.ReasonSignalsUnavailable
		if signalsErr != nil {
			reason += ": " + signalsErr.Error()
		}
	}
	if err := h.runs.Apply(ctx, run, decision.Event, reason); err != nil {
		return "", err
	}
	if decision.Event == domain.EventRegress {
		if err := h.folder.Archive(run, round); err != nil {
			return "", err
		}
	}
	return decision.Outcome, nil
}

func breaches(threshold domain.Threshold, verdict shareddomain.Verdict) []domain.SignalBreach {
	if verdict.Kind != shareddomain.VerdictDeny {
		return nil
	}
	var found []domain.SignalBreach
	for line := range strings.SplitSeq(verdict.Reason, "\n") {
		if strings.TrimSpace(line) != "" {
			found = append(found, domain.SignalBreach{Threshold: threshold, Reason: line})
		}
	}
	return found
}

func signalPayload(check domain.SignalCheck) json.RawMessage {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(check)
	return bytes.TrimRight(buf.Bytes(), "\n")
}
