package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/shared/application/eventbus"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	spanGateRun       = "gate.run"
	spanGateVerify    = "gate.verify"
	spanGateSetup     = "gate.setup"
	descGateFailed    = "a gate failed"
	metricGateSeconds = "harness.gate.duration"
	metricUnitSeconds = "s"
	reportErrorFormat = "\nfindings report: %v"
)

const (
	attrGate   = attribute.Key("harness.gate")
	attrResult = attribute.Key("harness.gate.result")
	attrCached = attribute.Key("harness.gate.cached")

	attrGateCount = attribute.Key("harness.gate.count")
	attrWorktree  = attribute.Key("harness.worktree")
	attrSetupStep = attribute.Key("harness.setup.step")
)

var (
	errUnknownReportVersion = errors.New("unknown findings report version")
	errStopped              = errors.New("another gate failed with an error")
)

type GateBeforePayload struct {
	Gate      string `json:"gate"`
	Run       string `json:"run"`
	InputHash string `json:"input_hash"`
}

type GateAfterPayload struct {
	GateBeforePayload
	Result     domain.Result `json:"result"`
	DurationMs int64         `json:"duration_ms"`
}

type Command struct {
	Repo     string
	Dir      string
	Setup    []domain.Setup
	Gates    []domain.Gate
	Parallel int
}

type SetupLine struct {
	Step     string
	Duration time.Duration
}

type Line struct {
	Gate     string
	Status   domain.Status
	Duration time.Duration
	Tail     string
	Need     string
}

type Result struct {
	Setup  []SetupLine
	Lines  []Line
	Failed bool
}

type ReadFile func(path string) ([]byte, error)

type Exists func(path string) (bool, error)

type Handler struct {
	hasher     domain.Hasher
	runner     domain.Runner
	evidence   domain.EvidenceRepository
	setups     domain.SetupRepository
	dispatcher shareddomain.Dispatcher
	criteria   domain.GateSource
	readFile   ReadFile
	exists     Exists
	tracer     trace.Tracer
	duration   metric.Float64Histogram
	now        func() time.Time
}

func NewHandler(
	hasher domain.Hasher,
	runner domain.Runner,
	evidence domain.EvidenceRepository,
	setups domain.SetupRepository,
	dispatcher shareddomain.Dispatcher,
	criteria domain.GateSource,
	readFile ReadFile,
	exists Exists,
	tracer trace.Tracer,
	meter metric.Meter,
	now func() time.Time,
) Handler {
	duration, _ := meter.Float64Histogram(
		metricGateSeconds,
		metric.WithUnit(metricUnitSeconds),
		metric.WithExplicitBucketBoundaries(eventbus.DurationBuckets()...),
	)
	return Handler{
		hasher: hasher, runner: runner, evidence: evidence, setups: setups, dispatcher: dispatcher,
		criteria: criteria, readFile: readFile, exists: exists, tracer: tracer, duration: duration, now: now,
	}
}

func (h Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	if h.criteria != nil {
		criterionGates, err := h.criteria.Gates(ctx)
		if err != nil {
			return Result{}, err
		}
		cmd.Gates = slices.Concat(cmd.Gates, criterionGates)
	}
	return h.HandleConfigured(ctx, cmd)
}

func (h Handler) HandleConfigured(ctx context.Context, cmd Command) (result Result, err error) {
	ctx, span := h.tracer.Start(ctx, spanGateVerify, trace.WithAttributes(
		attrGateCount.Int(len(cmd.Gates)), attrWorktree.String(cmd.Dir),
	))
	defer span.End()
	defer func() {
		switch {
		case err != nil:
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		case result.Failed:
			span.SetStatus(codes.Error, descGateFailed)
		}
	}()

	setup, err := h.runSetup(ctx, cmd)
	if err != nil {
		return Result{}, err
	}
	lines, err := h.schedule(ctx, cmd)
	if err != nil {
		return Result{}, err
	}
	result = Result{Setup: setup, Lines: lines}
	for _, line := range lines {
		result.Failed = result.Failed || !line.Status.Passed()
	}
	return result, nil
}

func (h Handler) runSetup(ctx context.Context, cmd Command) ([]SetupLine, error) {
	var lines []SetupLine
	for _, step := range cmd.Setup {
		hash, err := h.hasher.InputHash(ctx, cmd.Dir, step.Gate)
		if err != nil {
			return nil, err
		}
		current, err := h.setupCurrent(ctx, cmd, step, hash)
		if err != nil {
			return nil, err
		}
		if current {
			continue
		}
		line, err := h.runSetupStep(ctx, cmd, step, hash)
		if err != nil {
			return nil, err
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func (h Handler) runSetupStep(ctx context.Context, cmd Command, step domain.Setup, hash string) (line SetupLine, err error) {
	ctx, span := h.tracer.Start(ctx, spanGateSetup, trace.WithAttributes(attrSetupStep.String(step.ID)))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	outcome, err := h.runner.Run(ctx, cmd.Dir, step.Gate)
	if err != nil {
		return SetupLine{}, err
	}
	if outcome.Result != domain.ResultPass {
		return SetupLine{}, fmt.Errorf("%w: %s\n%s", domain.ErrSetupFailed, step.ID, outcome.Tail)
	}
	if err := h.setups.SavePass(ctx, cmd.Repo, step.ID, hash, h.now()); err != nil {
		return SetupLine{}, err
	}
	return SetupLine{Step: step.ID, Duration: outcome.Duration}, nil
}

func (h Handler) setupCurrent(ctx context.Context, cmd Command, step domain.Setup, hash string) (bool, error) {
	if step.Creates != "" {
		created, err := h.exists(filepath.Join(cmd.Dir, step.Creates))
		if err != nil || !created {
			return false, err
		}
	}
	last, ok, err := h.setups.LastPass(ctx, cmd.Repo, step.ID)
	return ok && last == hash, err
}

type finished struct {
	index int
	line  Line
	err   error
}

func (h Handler) schedule(ctx context.Context, cmd Command) ([]Line, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	gates := cmd.Gates
	index := make(map[string]int, len(gates))
	for i, gate := range gates {
		index[gate.ID] = i
	}
	lines := make([]Line, len(gates))
	done := make([]bool, len(gates))
	started := make([]bool, len(gates))
	results := make(chan finished)
	slots := max(cmd.Parallel, 1)
	running, remaining := 0, len(gates)
	var firstErr error

	for remaining > 0 {
		for progress := true; progress; {
			progress = false
			for i, gate := range gates {
				if started[i] || done[i] {
					continue
				}
				need, blocked, ready := needState(gate, index, lines, done)
				if blocked {
					lines[i] = Line{Gate: gate.ID, Status: domain.StatusSkipped, Need: need}
					done[i], progress = true, true
					remaining--
					continue
				}
				if !ready || running == slots || firstErr != nil {
					continue
				}
				started[i] = true
				running++
				go func() {
					line, err := h.runGate(ctx, cmd.Repo, cmd.Dir, gate)
					results <- finished{index: i, line: line, err: err}
				}()
			}
		}
		if running == 0 {
			break
		}
		f := <-results
		running--
		remaining--
		done[f.index] = true
		lines[f.index] = f.line
		if f.err != nil && firstErr == nil {
			firstErr = f.err
			cancel(errStopped)
		}
	}
	return lines, firstErr
}

func needState(gate domain.Gate, index map[string]int, lines []Line, done []bool) (need string, blocked, ready bool) {
	ready = true
	for _, need := range gate.Needs {
		i := index[need]
		switch {
		case !done[i]:
			ready = false
		case !lines[i].Status.Passed():
			return need, true, false
		}
	}
	return "", false, ready
}

func (h Handler) runGate(ctx context.Context, repo, dir string, gate domain.Gate) (line Line, err error) {
	ctx, span := h.tracer.Start(ctx, spanGateRun, trace.WithAttributes(attrGate.String(gate.ID)))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	hash, err := h.hasher.InputHash(ctx, dir, gate)
	if err != nil {
		return Line{}, err
	}
	found, ok, err := h.evidence.Find(ctx, repo, gate.ID, hash)
	if err != nil {
		return Line{}, err
	}
	if ok && found.Result == domain.ResultPass {
		span.SetAttributes(attrResult.String(string(found.Result)), attrCached.Bool(true))
		return Line{Gate: gate.ID, Status: domain.StatusCached, Duration: found.Duration, Tail: found.Tail}, nil
	}
	span.SetAttributes(attrCached.Bool(false))

	before := GateBeforePayload{Gate: gate.ID, Run: gate.Run, InputHash: hash}
	verdict, err := h.dispatch(ctx, shareddomain.EventGateBefore, dir, before)
	if err != nil {
		return Line{}, err
	}
	if verdict.Kind == shareddomain.VerdictDeny {
		span.SetAttributes(attrResult.String(string(domain.ResultVetoed)))
		return Line{Gate: gate.ID, Status: domain.StatusVetoed, Tail: verdict.Reason}, nil
	}

	outcome, err := h.runner.Run(ctx, dir, gate)
	if err != nil {
		return Line{}, err
	}
	if err := context.Cause(ctx); errors.Is(err, errStopped) {
		return Line{}, err
	}
	if gate.Baseline != "" {
		if outcome, err = h.judgeBaseline(dir, gate, outcome); err != nil {
			return Line{}, err
		}
	}
	err = h.evidence.Save(ctx, domain.Evidence{
		Repo: repo, GateID: gate.ID, InputHash: hash, Result: outcome.Result,
		Duration: outcome.Duration, Tail: outcome.Tail, At: h.now(),
	})
	if err != nil {
		return Line{}, err
	}
	after := GateAfterPayload{GateBeforePayload: before, Result: outcome.Result, DurationMs: outcome.Duration.Milliseconds()}
	if _, err = h.dispatch(ctx, shareddomain.EventGateAfter, dir, after); err != nil {
		return Line{}, err
	}

	attrs := []attribute.KeyValue{attrGate.String(gate.ID), attrResult.String(string(outcome.Result))}
	span.SetAttributes(attrResult.String(string(outcome.Result)))
	h.duration.Record(ctx, outcome.Duration.Seconds(), metric.WithAttributes(attrs...))
	return Line{Gate: gate.ID, Status: statusOf(outcome.Result), Duration: outcome.Duration, Tail: outcome.Tail}, nil
}

func (h Handler) dispatch(ctx context.Context, event shareddomain.EventName, dir string, payload any) (shareddomain.Verdict, error) {
	raw, _ := json.Marshal(payload)
	dispatched, err := h.dispatcher.Dispatch(ctx, shareddomain.Envelope{
		Version: shareddomain.EnvelopeVersion, Event: event, Cwd: dir, Payload: raw,
	})
	return dispatched.Verdict, err
}

func (h Handler) judgeBaseline(dir string, gate domain.Gate, outcome domain.Outcome) (domain.Outcome, error) {
	if outcome.Result != domain.ResultPass && strings.TrimSpace(outcome.Stdout) == "" {
		return outcome, nil
	}
	found, err := parseReport([]byte(outcome.Stdout))
	if err != nil {
		outcome.Result = domain.ResultFail
		outcome.Tail += fmt.Sprintf(reportErrorFormat, err)
		return outcome, nil
	}
	var known []shareddomain.Finding
	raw, err := h.readFile(filepath.Join(dir, gate.Baseline))
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return domain.Outcome{}, err
	default:
		if known, err = parseReport(raw); err != nil {
			return domain.Outcome{}, fmt.Errorf("baseline %s: %w", gate.Baseline, err)
		}
	}
	unexplainedExit := outcome.Result != domain.ResultPass && len(domain.NewFindings(found, nil)) == 0
	outcome.Result = domain.ResultPass
	if outcome.TimedOut || unexplainedExit || len(domain.NewFindings(found, known)) > 0 {
		outcome.Result = domain.ResultFail
	}
	return outcome, nil
}

func parseReport(raw []byte) ([]shareddomain.Finding, error) {
	var report shareddomain.Report
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, err
	}
	if report.Version != shareddomain.ReportVersion {
		return nil, fmt.Errorf("%w: %d", errUnknownReportVersion, report.Version)
	}
	return report.Findings, nil
}

func statusOf(result domain.Result) domain.Status {
	if result == domain.ResultPass {
		return domain.StatusPass
	}
	return domain.StatusFail
}
