package waitpr_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/run/application/command/waitpr"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	spanName = "run.wait_pr"
	runID    = "20261111-142102-fix-106"
	prURL    = "https://github.com/90p-labs/harness/pull/107"
)

var key = domain.RunKey{Repo: "github.com/90p-labs/harness", Worktree: "/work/harness"}

type fakeRepository struct {
	stored    domain.Run
	activeErr error
	saveErr   error
	saved     []domain.Run
}

func (f *fakeRepository) Active(context.Context, domain.RunKey) (domain.Run, error) {
	return f.stored, f.activeErr
}

func (f *fakeRepository) Get(context.Context, string) (domain.Run, error) { return f.stored, nil }

func (f *fakeRepository) ListByPhase(context.Context, string, []domain.Phase) ([]domain.Run, error) {
	return nil, nil
}

func (f *fakeRepository) Save(_ context.Context, run domain.Run) (domain.Run, error) {
	f.saved = append(f.saved, run)
	return run, f.saveErr
}

func storedRun(phase domain.Phase) domain.Run {
	return domain.Run{ID: runID, Key: key, Phase: phase}
}

func TestWaitPRSpanCarriesTheRun(t *testing.T) {
	spans := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test")
	handler := waitpr.NewHandler(&fakeRepository{stored: storedRun(domain.PhaseShipping)}, tracer)

	if err := handler.Handle(context.Background(), waitpr.Command{Key: key, PRURL: prURL}); err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	for _, kv := range spans.Ended()[0].Attributes() {
		got[string(kv.Key)] = kv.Value.AsString()
	}
	if got["harness.run.id"] != runID || got["harness.phase.from"] != string(domain.PhaseShipping) {
		t.Errorf("attributes = %v, want run id %q and phase shipping", got, runID)
	}
}

func TestWaitPR(t *testing.T) {
	errStore := errors.New("store down")
	cases := []struct {
		name      string
		repo      *fakeRepository
		url       string
		wantErr   error
		wantSaved bool
	}{
		{"records the URL in shipping", &fakeRepository{stored: storedRun(domain.PhaseShipping)}, prURL, nil, true},
		{"refuses outside shipping", &fakeRepository{stored: storedRun(domain.PhaseWorking)}, prURL, domain.ErrNotShipping, false},
		{"refuses an invalid URL", &fakeRepository{stored: storedRun(domain.PhaseShipping)}, "https://example.com/1", shareddomain.ErrInvalidPRURL, false},
		{"propagates a missing run", &fakeRepository{activeErr: domain.ErrRunNotFound}, prURL, domain.ErrRunNotFound, false},
		{"propagates a save error", &fakeRepository{stored: storedRun(domain.PhaseShipping), saveErr: errStore}, prURL, errStore, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spans := tracetest.NewSpanRecorder()
			tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test")
			handler := waitpr.NewHandler(tc.repo, tracer)

			err := handler.Handle(context.Background(), waitpr.Command{Key: key, PRURL: tc.url})

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantSaved {
				if len(tc.repo.saved) != 1 || tc.repo.saved[0].PRURL != tc.url {
					t.Errorf("saved = %+v, want one run with PRURL %q", tc.repo.saved, tc.url)
				}
			} else if len(tc.repo.saved) != 0 {
				t.Errorf("saved = %+v, want none", tc.repo.saved)
			}
			ended := spans.Ended()
			if len(ended) != 1 || ended[0].Name() != spanName {
				t.Fatalf("spans = %+v, want one %s", ended, spanName)
			}
			if gotErr := ended[0].Status().Code == codes.Error; gotErr != (tc.wantErr != nil) {
				t.Errorf("span error status = %v, want %v", gotErr, tc.wantErr != nil)
			}
		})
	}
}
