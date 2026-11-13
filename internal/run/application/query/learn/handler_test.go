package learn_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/run/application/query/learn"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/telemetry"
)

const spanLearn = "run.learn"

const runID = "20261113-004016-turn-repeated-corrections-into-durable-p"

var key = domain.RunKey{Repo: "github.com/90p-labs/harness", Worktree: "/work/harness"}

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
	rows []domain.Transitioned
}

func (f fakeLog) Transitions(context.Context, string) ([]domain.Transitioned, error) {
	return f.rows, nil
}

func (fakeLog) RepeatClasses(context.Context, string) ([]domain.ClassCount, error) {
	return nil, nil
}

type fakeFindings struct {
	findings []shareddomain.ReviewFinding
	err      error
}

func (f fakeFindings) Read(context.Context, domain.Run) ([]shareddomain.ReviewFinding, error) {
	return f.findings, f.err
}

func newHandler(reader fakeFindings, log fakeLog) (learn.Handler, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	run := domain.Run{ID: runID, Key: key, Phase: domain.PhaseShipping}
	return learn.NewHandler(fakeRepository{run: run}, log, reader, tracer), recorder
}

func TestLearnHandler(t *testing.T) {
	t.Run("lists the candidates of the run", func(t *testing.T) {
		reader := fakeFindings{findings: []shareddomain.ReviewFinding{
			{ID: "S1", Verdict: shareddomain.FindingReal, Lens: "standards", Location: "a.go:3", Text: "name the constant"},
			{ID: "P1", Verdict: shareddomain.FindingReal, Lens: "spec", Text: "one-off bug"},
			{ID: "G1", Verdict: shareddomain.FindingReal, Lens: "spec", Location: "b.go:9", Text: "run the linter"},
		}}
		log := fakeLog{rows: []domain.Transitioned{
			{Event: domain.EventRequestRework, Class: domain.ReworkGateCheckable, Reason: "fix G1"},
		}}
		handler, _ := newHandler(reader, log)

		got, err := handler.Handle(context.Background(), learn.Query{Key: key})

		want := learn.Learn{ID: runID, Candidates: []domain.Candidate{
			{Source: domain.SourceStandards, ID: "S1", Location: "a.go:3", Text: "name the constant"},
			{Source: domain.SourceGateCheckable, ID: "G1", Location: "b.go:9", Text: "run the linter"},
		}}
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, %v, want %+v", got, err, want)
		}
	})

	t.Run("finds the run by id", func(t *testing.T) {
		handler, _ := newHandler(fakeFindings{}, fakeLog{})

		got, err := handler.Handle(context.Background(), learn.Query{RunID: runID})

		if err != nil || got.ID != runID {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("a missing findings file gives no candidate and no error", func(t *testing.T) {
		missing := errors.Join(domain.ErrNoFindingsFile)
		handler, recorder := newHandler(fakeFindings{err: missing}, fakeLog{})

		got, err := handler.Handle(context.Background(), learn.Query{Key: key})

		spans := recorder.Ended()
		if err != nil || got.ID != runID || len(got.Candidates) != 0 {
			t.Fatalf("got %+v, %v", got, err)
		}
		if len(spans) != 1 || spans[0].Status().Code == codes.Error {
			t.Fatalf("spans = %+v", spans)
		}
	})

	t.Run("another reader error is returned and recorded on the span", func(t *testing.T) {
		boom := errors.New("findings.md: line 3: bad severity")
		handler, recorder := newHandler(fakeFindings{err: boom}, fakeLog{})

		_, err := handler.Handle(context.Background(), learn.Query{Key: key})

		spans := recorder.Ended()
		if !errors.Is(err, boom) || len(spans) != 1 || spans[0].Status().Code != codes.Error || len(spans[0].Events()) == 0 {
			t.Fatalf("err = %v, spans = %+v", err, spans)
		}
	})

	t.Run("the span is run.learn and carries the run id", func(t *testing.T) {
		handler, recorder := newHandler(fakeFindings{}, fakeLog{})

		if _, err := handler.Handle(context.Background(), learn.Query{Key: key}); err != nil {
			t.Fatal(err)
		}

		spans := recorder.Ended()
		if len(spans) != 1 || spans[0].Name() != spanLearn {
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
		handler, recorder := newHandler(fakeFindings{}, fakeLog{})

		_, err := handler.Handle(context.Background(), learn.Query{RunID: "other"})

		spans := recorder.Ended()
		if !errors.Is(err, domain.ErrRunNotFound) || len(spans) != 1 || spans[0].Status().Code != codes.Error {
			t.Fatalf("err = %v, spans = %+v", err, spans)
		}
	})
}
