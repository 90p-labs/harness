package transition

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	spanTransition = "run.transition"
	byReasonFormat = "approved by %s: %s"
)

const (
	attrRunID     = attribute.Key("harness.run.id")
	attrEvent     = attribute.Key("harness.event")
	attrPhaseFrom = attribute.Key("harness.phase.from")
	attrPhaseTo   = attribute.Key("harness.phase.to")

	attrReviewRound    = attribute.Key("harness.review.round")
	attrReworkClass    = attribute.Key("harness.rework.class")
	attrReworkSeverity = attribute.Key("harness.rework.severity")
)

var gatedEvents = map[domain.Event]bool{
	domain.EventSubmitPlan:  true,
	domain.EventAmend:       true,
	domain.EventApprovePlan: true,
}

type Command struct {
	Key     domain.RunKey
	Event   domain.Event
	Reason  string
	By      string
	PRURL   string
	Class   domain.ReworkClass
	RunID   string
	Version int
}

type Flow struct {
	repo      domain.RunRepository
	hooks     domain.TransitionHooks
	gate      domain.PlanGate
	approvals domain.ApprovalRepository
	policy    domain.Policy
	clock     func() time.Time
	tracer    trace.Tracer
	runTrace  domain.RunTrace
}

func NewFlow(repo domain.RunRepository, hooks domain.TransitionHooks, gate domain.PlanGate, approvals domain.ApprovalRepository, policy domain.Policy, clock func() time.Time, tracer trace.Tracer, runTrace domain.RunTrace) Flow {
	return Flow{repo: repo, hooks: hooks, gate: gate, approvals: approvals, policy: policy, clock: clock, tracer: tracer, runTrace: runTrace}
}

func (f Flow) Move(ctx context.Context, run domain.Run, cmd domain.Command) (err error) {
	ctx = f.runTrace.Under(ctx, run)
	ctx, span := f.tracer.Start(ctx, spanTransition, trace.WithAttributes(
		attrRunID.String(run.ID),
		attrEvent.String(string(cmd.Event)),
		attrPhaseFrom.String(string(run.Phase)),
	))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	cmd.At = f.clock()
	if _, err := run.Apply(cmd, f.policy); err != nil {
		return err
	}
	policy := f.policy
	if gatedEvents[cmd.Event] {
		submission, err := f.gate.Submit(ctx, run)
		if err != nil {
			return err
		}
		cmd.Track = submission.Track
		if cmd.Event == domain.EventSubmitPlan && submission.ApprovalWaived {
			policy.ApprovalNeeded = false
		}
		if f.amendSkipsApproval(ctx, run, cmd, policy, submission) {
			policy.ApprovalNeeded = false
		}
		if cmd.Event == domain.EventApprovePlan && submission.QuestionsOpen {
			return domain.ErrOpenQuestions
		}
	}
	var hash string
	if cmd.Event == domain.EventApprovePlan {
		if cmd, hash, err = f.approved(ctx, run, cmd); err != nil {
			return err
		}
	}
	next, err := run.Apply(cmd, policy)
	if err != nil {
		return err
	}
	if err := f.hooks.Before(ctx, run, cmd, next.Phase); err != nil {
		return err
	}
	if _, err := f.repo.Save(ctx, next); err != nil {
		return err
	}
	if cmd.Event == domain.EventApprovePlan {
		if err := f.approvals.Save(ctx, run.ID, hash, cmd.At); err != nil {
			return err
		}
		if err := f.gate.Keep(run); err != nil {
			return err
		}
	}
	f.hooks.After(ctx, run, next)
	if next.Phase == domain.PhaseDone || next.Phase == domain.PhaseAbandoned {
		f.runTrace.End(ctx, next)
	}
	span.SetAttributes(attrPhaseTo.String(string(next.Phase)), attrReviewRound.Int(next.ReviewRound))
	if cmd.Event.IsRework() {
		span.SetAttributes(reworkAttributes(cmd)...)
	}
	return nil
}

func reworkAttributes(cmd domain.Command) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	if cmd.Class != "" {
		attrs = append(attrs, attrReworkClass.String(string(cmd.Class)))
	}
	if severity := domain.HighestSeverity(cmd.Reason); severity != "" {
		attrs = append(attrs, attrReworkSeverity.String(string(severity)))
	}
	return attrs
}

func (f Flow) approved(ctx context.Context, run domain.Run, cmd domain.Command) (domain.Command, string, error) {
	hash, err := f.gate.Hash(run)
	if err != nil {
		return cmd, "", err
	}
	if cmd.By != "" {
		cmd.Reason = fmt.Sprintf(byReasonFormat, cmd.By, cmd.Reason)
		return cmd, hash, nil
	}
	recorded, err := f.approvals.Has(ctx, run.ID, hash)
	if err != nil {
		return cmd, "", err
	}
	if !recorded {
		return cmd, "", domain.ErrApprovalNotRecorded
	}
	return cmd, hash, nil
}

func (f Flow) amendSkipsApproval(ctx context.Context, run domain.Run, cmd domain.Command, policy domain.Policy, submission domain.Submission) bool {
	return cmd.Event == domain.EventAmend && policy.ApprovalNeeded && run.Phase == domain.PhaseWorking && run.LiveRound == 0 && f.amendSkipsApprovalCase(ctx, run, submission)
}

func (f Flow) amendSkipsApprovalCase(ctx context.Context, run domain.Run, submission domain.Submission) bool {
	hash, ok, err := f.approvals.Latest(ctx, run.ID)
	if err != nil {
		trace.SpanFromContext(ctx).RecordError(err)
		return false
	}
	if !ok {
		return submission.ApprovalWaived
	}
	kept, err := f.gate.StructureKept(run, hash)
	if err != nil {
		trace.SpanFromContext(ctx).RecordError(err)
		return false
	}
	return kept
}

type Handler struct {
	repo     domain.RunRepository
	flow     Flow
	findings domain.FindingsReader
	policy   domain.Policy
}

func NewHandler(repo domain.RunRepository, flow Flow, findings domain.FindingsReader, policy domain.Policy) Handler {
	return Handler{repo: repo, flow: flow, findings: findings, policy: policy}
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	run, err := h.load(ctx, cmd)
	if err != nil {
		return err
	}
	if cmd.Event == domain.EventRequestRework {
		findings, err := h.findings.Read(ctx, run)
		if err != nil {
			return err
		}
		if cmd.Event, err = domain.ReworkEvent(findings, h.policy, cmd.Reason); err != nil {
			return err
		}
	}
	return h.flow.Move(ctx, run, domain.Command{Event: cmd.Event, Reason: cmd.Reason, By: cmd.By, PRURL: cmd.PRURL, Class: cmd.Class})
}

func (h Handler) load(ctx context.Context, cmd Command) (domain.Run, error) {
	if cmd.RunID == "" {
		return h.repo.Active(ctx, cmd.Key)
	}
	run, err := h.repo.Get(ctx, cmd.RunID)
	if err != nil {
		return domain.Run{}, err
	}
	if run.Version != cmd.Version {
		return domain.Run{}, fmt.Errorf("%w: %s at version %d, want %d", domain.ErrStaleRun, run.ID, run.Version, cmd.Version)
	}
	return run, nil
}
