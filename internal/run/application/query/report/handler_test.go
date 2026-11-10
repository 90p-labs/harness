package report_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/run/application/query/report"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/telemetry"
)

const spanReport = "run.report"

const runID = "20261110-082002-make-the-run-traces-answer-the-audit-que"

var (
	key     = domain.RunKey{Repo: "github.com/90p-labs/harness", Worktree: "/work/harness"}
	started = time.Date(2026, 11, 10, 8, 20, 0, 0, time.UTC)
)

type fakeRepository struct {
	run domain.Run
}

func (f fakeRepository) Active(_ context.Context, k domain.RunKey) (domain.Run, error) {
	if f.run.Key != k {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return f.run, nil
}

func (f fakeRepository) Get(_ context.Context, id string) (domain.Run, error) {
	if f.run.ID != id {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return f.run, nil
}

func (fakeRepository) ListByPhase(context.Context, string, []domain.Phase) ([]domain.Run, error) {
	return nil, nil
}

func (fakeRepository) Save(context.Context, domain.Run) (domain.Run, error) {
	return domain.Run{}, errors.New("a query never saves")
}

type fakeLog struct {
	rows    []domain.Transitioned
	repeats []domain.ClassCount
}

func (f fakeLog) Transitions(context.Context, string) ([]domain.Transitioned, error) {
	return f.rows, nil
}

func (f fakeLog) RepeatClasses(context.Context, string) ([]domain.ClassCount, error) {
	return f.repeats, nil
}

func at(minutes float64) time.Time {
	return started.Add(time.Duration(minutes * float64(time.Minute)))
}

func fakeTraceID(id string) string { return "trace-of-" + id }

func newHandler(run domain.Run, log fakeLog, now time.Time) (report.Handler, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	return report.NewHandler(fakeRepository{run: run}, log, func() time.Time { return now }, fakeTraceID, tracer), recorder
}

func TestReportCountsAndMinutes(t *testing.T) {
	run := domain.Run{ID: runID, Key: key, Phase: domain.PhaseReviewing, StartedAt: started, StopBlocksTotal: 2}
	log := fakeLog{rows: []domain.Transitioned{
		{Event: domain.EventStart, To: domain.PhasePlanning, At: at(0)},
		{Event: domain.EventSubmitPlan, From: domain.PhasePlanning, To: domain.PhaseAwaitingApproval, At: at(18)},
		{Event: domain.EventApprovePlan, From: domain.PhaseAwaitingApproval, To: domain.PhaseWorking, At: at(18.5)},
		{Event: domain.EventSubmitForReview, From: domain.PhaseWorking, To: domain.PhaseReviewing, At: at(30)},
		{Event: domain.EventRequestRework, From: domain.PhaseReviewing, To: domain.PhaseWorking, Class: domain.ReworkImplementation, At: at(36)},
		{Event: domain.EventAmend, From: domain.PhaseWorking, To: domain.PhasePlanning, At: at(40)},
		{Event: domain.EventSubmitPlan, From: domain.PhasePlanning, To: domain.PhaseAwaitingApproval, At: at(41)},
		{Event: domain.EventApprovePlan, From: domain.PhaseAwaitingApproval, To: domain.PhaseWorking, At: at(42)},
		{Event: domain.EventSubmitForReview, From: domain.PhaseWorking, To: domain.PhaseReviewing, At: at(50)},
	}}
	handler, _ := newHandler(run, log, at(54))

	got, err := handler.Handle(context.Background(), report.Query{Key: key})

	want := report.Report{
		ID: runID, Phase: domain.PhaseReviewing, ReworkRounds: 1, Amends: 1, StopBlocks: 2,
		Classes: []domain.ClassCount{{Class: domain.ReworkImplementation, Count: 1}},
		Minutes: []report.PhaseMinutes{
			{Phase: domain.PhasePlanning, Minutes: 19},
			{Phase: domain.PhaseAwaitingApproval, Minutes: 1.5},
			{Phase: domain.PhaseWorking, Minutes: 23.5},
			{Phase: domain.PhaseReviewing, Minutes: 10},
		},
		TraceID: fakeTraceID(runID),
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v, want %+v", got, err, want)
	}
}

func TestReportClosedRunDoesNotCountTheTail(t *testing.T) {
	run := domain.Run{ID: runID, Key: key, Phase: domain.PhaseDone, StartedAt: started}
	log := fakeLog{rows: []domain.Transitioned{
		{Event: domain.EventStart, To: domain.PhasePlanning, At: at(0)},
		{Event: domain.EventAbandon, From: domain.PhasePlanning, To: domain.PhaseAbandoned, At: at(5)},
	}}
	handler, _ := newHandler(run, log, at(500))

	got, err := handler.Handle(context.Background(), report.Query{RunID: runID})

	want := []report.PhaseMinutes{{Phase: domain.PhasePlanning, Minutes: 5}}
	if err != nil || !reflect.DeepEqual(got.Minutes, want) {
		t.Fatalf("minutes = %+v, %v, want %+v", got.Minutes, err, want)
	}
}

func TestReportSpan(t *testing.T) {
	t.Run("the span carries the run id", func(t *testing.T) {
		run := domain.Run{ID: runID, Key: key, Phase: domain.PhasePlanning, StartedAt: started}
		handler, recorder := newHandler(run, fakeLog{}, at(1))

		if _, err := handler.Handle(context.Background(), report.Query{Key: key}); err != nil {
			t.Fatal(err)
		}

		spans := recorder.Ended()
		if len(spans) != 1 || spans[0].Name() != spanReport {
			t.Fatalf("spans = %+v", spans)
		}
		found := false
		for _, kv := range spans[0].Attributes() {
			found = found || (kv.Key == telemetry.AttrRunID && kv.Value.AsString() == runID)
		}
		if !found {
			t.Fatalf("attributes = %+v", spans[0].Attributes())
		}
	})

	t.Run("a missing run marks the span as an error", func(t *testing.T) {
		handler, recorder := newHandler(domain.Run{ID: runID, Key: key}, fakeLog{}, at(1))

		_, err := handler.Handle(context.Background(), report.Query{RunID: "other"})

		spans := recorder.Ended()
		if !errors.Is(err, domain.ErrRunNotFound) || len(spans) != 1 || spans[0].Status().Code != codes.Error {
			t.Fatalf("err = %v, spans = %+v", err, spans)
		}
	})
}

func TestReportRepeatClasses(t *testing.T) {
	want := []domain.ClassCount{{Class: domain.ReworkMinor, Count: 2}}
	handler, _ := newHandler(domain.Run{}, fakeLog{repeats: want}, at(0))

	got, err := handler.RepeatClasses(context.Background(), key.Repo)

	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %v", got, err)
	}
}
