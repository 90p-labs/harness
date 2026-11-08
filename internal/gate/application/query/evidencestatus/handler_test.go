package evidencestatus_test

import (
	"context"
	"errors"
	"maps"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jorge07/harness/internal/gate/application/query/evidencestatus"
	"github.com/jorge07/harness/internal/gate/domain"
)

const (
	repo     = "github.com/jorge07/harness"
	workDir  = "/work/harness"
	spanName = "gate.evidence_status"
	testGate = "go-test"
	lintGate = "lint"
	typeGate = "types"
)

var errInjected = errors.New("injected")

type fakeHasher struct{ err error }

func (f fakeHasher) InputHash(_ context.Context, _ string, gate domain.Gate) (string, error) {
	if len(gate.Paths) == 0 {
		return "tree-all", f.err
	}
	return "tree-" + gate.Paths[0], f.err
}

type fakeEvidence struct {
	records map[string]domain.Evidence
	err     error
}

func (f fakeEvidence) Find(_ context.Context, gotRepo, gateID, hash string) (domain.Evidence, bool, error) {
	if gotRepo != repo {
		return domain.Evidence{}, false, nil
	}
	e, ok := f.records[gateID+"@"+hash]
	return e, ok, f.err
}

func (fakeEvidence) Save(context.Context, domain.Evidence) error { return nil }

func TestEvidenceStatus(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test")
	records := fakeEvidence{records: map[string]domain.Evidence{
		testGate + "@tree-all": {Result: domain.ResultPass},
		lintGate + "@tree-old": {Result: domain.ResultPass},
		lintGate + "@tree-src": {Result: domain.ResultFail},
	}}
	handler := evidencestatus.NewHandler(fakeHasher{}, records, tracer)
	gates := []domain.Gate{
		{ID: testGate},
		{ID: lintGate, Paths: []string{"src"}},
		{ID: typeGate},
	}

	got, err := handler.Handle(context.Background(), evidencestatus.Query{Repo: repo, Dir: workDir, Gates: gates})

	if err != nil {
		t.Fatal(err)
	}
	want := map[string]domain.Result{testGate: domain.ResultPass, lintGate: domain.ResultFail}
	if !maps.Equal(got, want) {
		t.Fatalf("status = %v, want %v", got, want)
	}
	if ended := spans.Ended(); len(ended) != 1 || ended[0].Name() != spanName {
		t.Fatalf("spans = %v, want one %s", ended, spanName)
	}
}

func TestEvidenceStatusErrors(t *testing.T) {
	tracer := sdktrace.NewTracerProvider().Tracer("test")
	cases := map[string]evidencestatus.Handler{
		"hasher": evidencestatus.NewHandler(fakeHasher{err: errInjected}, fakeEvidence{}, tracer),
		"find":   evidencestatus.NewHandler(fakeHasher{}, fakeEvidence{err: errInjected}, tracer),
	}
	for name, handler := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := handler.Handle(context.Background(), evidencestatus.Query{Repo: repo, Dir: workDir, Gates: []domain.Gate{{ID: testGate}}})

			if !errors.Is(err, errInjected) {
				t.Fatalf("error = %v, want the injected error", err)
			}
		})
	}
}
