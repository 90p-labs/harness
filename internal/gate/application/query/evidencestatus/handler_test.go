package evidencestatus_test

import (
	"context"
	"errors"
	"maps"
	"strings"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/gate/application/query/evidencestatus"
	"github.com/90p-labs/harness/internal/gate/domain"
)

const (
	repo     = "github.com/90p-labs/harness"
	workDir  = "/work/harness"
	spanName = "gate.evidence_status"
	testGate = "go-test"
	lintGate = "lint"
	typeGate = "types"
)

var errInjected = errors.New("injected")

type fakeHasher struct {
	err   error
	calls int
	// short drops this many hashes from the result.
	short int
	// extra appends this many hashes to the result.
	extra int
}

func (f *fakeHasher) InputHashes(_ context.Context, _ string, gates []domain.Gate) ([]string, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	hashes := make([]string, 0, len(gates))
	for _, gate := range gates {
		// Distinct per gate, so a misaligned gate and hash index finds no record.
		hashes = append(hashes, "tree-"+gate.ID+"-"+strings.Join(gate.Paths, ","))
	}
	for i := 0; i < f.extra; i++ {
		hashes = append(hashes, "tree-extra")
	}
	return hashes[:max(0, len(hashes)-f.short)], nil
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
		testGate + "@tree-go-test-": {Result: domain.ResultPass},
		lintGate + "@tree-lint-old": {Result: domain.ResultPass},
		lintGate + "@tree-lint-src": {Result: domain.ResultFail},
	}}
	handler := evidencestatus.NewHandler(&fakeHasher{}, records, tracer)

	got, err := handler.Handle(context.Background(), evidencestatus.Query{Repo: repo, Dir: workDir, Gates: threeGates()})

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

func threeGates() []domain.Gate {
	return []domain.Gate{
		{ID: testGate},
		{ID: lintGate, Paths: []string{"src"}},
		{ID: typeGate},
	}
}

func TestEvidenceStatusHashesOnce(t *testing.T) {
	tracer := sdktrace.NewTracerProvider().Tracer("test")
	hasher := &fakeHasher{}
	records := fakeEvidence{records: map[string]domain.Evidence{
		testGate + "@tree-go-test-":    {Result: domain.ResultPass},
		lintGate + "@tree-lint-src":    {Result: domain.ResultFail},
		typeGate + "@tree-types-stale": {Result: domain.ResultPass},
	}}
	handler := evidencestatus.NewHandler(hasher, records, tracer)

	got, err := handler.Handle(context.Background(), evidencestatus.Query{Repo: repo, Dir: workDir, Gates: threeGates()})

	if err != nil {
		t.Fatal(err)
	}
	if hasher.calls != 1 {
		t.Fatalf("InputHashes calls = %d, want 1", hasher.calls)
	}
	want := map[string]domain.Result{testGate: domain.ResultPass, lintGate: domain.ResultFail}
	if !maps.Equal(got, want) {
		t.Fatalf("status = %v, want %v", got, want)
	}
}

func TestEvidenceStatusHashCount(t *testing.T) {
	tracer := sdktrace.NewTracerProvider().Tracer("test")
	cases := map[string]struct {
		hasher *fakeHasher
		want   string
	}{
		"fewer hashes than gates":       {&fakeHasher{short: 1}, "hasher returned 2 hashes for 3 gates"},
		"more hashes than gates":        {&fakeHasher{extra: 1}, "hasher returned 4 hashes for 3 gates"},
		"no hashes at all":              {&fakeHasher{short: 5}, "hasher returned 0 hashes for 3 gates"},
		"a short list and an extra one": {&fakeHasher{short: 2, extra: 1}, "hasher returned 2 hashes for 3 gates"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			handler := evidencestatus.NewHandler(tc.hasher, fakeEvidence{}, tracer)

			_, err := handler.Handle(context.Background(), evidencestatus.Query{Repo: repo, Dir: workDir, Gates: threeGates()})

			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestEvidenceStatusErrors(t *testing.T) {
	tracer := sdktrace.NewTracerProvider().Tracer("test")
	cases := map[string]evidencestatus.Handler{
		"hasher": evidencestatus.NewHandler(&fakeHasher{err: errInjected}, fakeEvidence{}, tracer),
		"find":   evidencestatus.NewHandler(&fakeHasher{}, fakeEvidence{err: errInjected}, tracer),
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
