package verify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
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
	metricGateSeconds = "harness.gate.duration"
	metricUnitSeconds = "s"
	reportErrorFormat = "\nfindings report: %v"
)

const (
	attrGate   = attribute.Key("harness.gate")
	attrResult = attribute.Key("harness.gate.result")
	attrCached = attribute.Key("harness.gate.cached")
)

var errUnknownReportVersion = errors.New("unknown findings report version")

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
	Repo  string
	Dir   string
	Gates []domain.Gate
}

type Line struct {
	Gate     string
	Status   domain.Status
	Duration time.Duration
	Tail     string
}

type Result struct {
	Lines  []Line
	Failed bool
}

type ReadFile func(path string) ([]byte, error)

type Handler struct {
	hasher     domain.Hasher
	runner     domain.Runner
	evidence   domain.EvidenceRepository
	dispatcher shareddomain.Dispatcher
	criteria   domain.GateSource
	readFile   ReadFile
	tracer     trace.Tracer
	duration   metric.Float64Histogram
	now        func() time.Time
}

func NewHandler(
	hasher domain.Hasher,
	runner domain.Runner,
	evidence domain.EvidenceRepository,
	dispatcher shareddomain.Dispatcher,
	criteria domain.GateSource,
	readFile ReadFile,
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
		hasher: hasher, runner: runner, evidence: evidence, dispatcher: dispatcher,
		criteria: criteria, readFile: readFile, tracer: tracer, duration: duration, now: now,
	}
}

func (h Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	gates := cmd.Gates
	if h.criteria != nil {
		criterionGates, err := h.criteria.Gates(ctx)
		if err != nil {
			return Result{}, err
		}
		gates = slices.Concat(cmd.Gates, criterionGates)
	}
	return h.run(ctx, cmd.Repo, cmd.Dir, gates)
}

func (h Handler) HandleConfigured(ctx context.Context, cmd Command) (Result, error) {
	return h.run(ctx, cmd.Repo, cmd.Dir, cmd.Gates)
}

func (h Handler) run(ctx context.Context, repo, dir string, gates []domain.Gate) (Result, error) {
	var result Result
	for _, gate := range gates {
		line, err := h.runGate(ctx, repo, dir, gate)
		if err != nil {
			return Result{}, err
		}
		result.Lines = append(result.Lines, line)
		if line.Status == domain.StatusFail || line.Status == domain.StatusVetoed {
			result.Failed = true
		}
	}
	return result, nil
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
