package decidestop_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jorge07/harness/internal/run/application/command/decidestop"
	"github.com/jorge07/harness/internal/run/domain"
)

const (
	spanName  = "run.decide_stop"
	attrBlock = "harness.stop.block"

	runID         = "20261108-011805-add-login"
	maxStopBlocks = 3
)

var key = domain.RunKey{Repo: "github.com/jorge07/harness", Worktree: "/work/harness"}

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

func storedRun(phase domain.Phase, stopBlocks int) domain.Run {
	return domain.Run{ID: runID, Key: key, Phase: phase, StopBlocks: stopBlocks}
}

func TestDecideStop(t *testing.T) {
	cases := []struct {
		name         string
		repo         *fakeRepository
		agentRunning bool
		wantBlock    bool
		wantSaved    int
	}{
		{"no active run allows", &fakeRepository{activeErr: domain.ErrRunNotFound}, false, false, 0},
		{"a phase that holds the turn blocks and counts the block", &fakeRepository{stored: storedRun(domain.PhaseWorking, 1)}, false, true, 2},
		{"a running agent allows", &fakeRepository{stored: storedRun(domain.PhaseWorking, 0)}, true, false, 0},
		{"the block limit allows", &fakeRepository{stored: storedRun(domain.PhaseWorking, maxStopBlocks)}, false, false, 0},
		{"a phase that does not hold the turn allows", &fakeRepository{stored: storedRun(domain.PhaseAwaitingApproval, 0)}, false, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spans := tracetest.NewSpanRecorder()
			tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test")
			handler := decidestop.NewHandler(tc.repo, domain.Policy{MaxStopBlocks: maxStopBlocks}, tracer)

			result, err := handler.Handle(context.Background(), decidestop.Command{Key: key, AgentRunning: tc.agentRunning})

			if err != nil {
				t.Fatal(err)
			}
			if result.Block != tc.wantBlock || (result.Reason != "") != tc.wantBlock {
				t.Errorf("result = %+v, want block %v", result, tc.wantBlock)
			}
			if tc.wantSaved == 0 {
				if len(tc.repo.saved) != 0 {
					t.Errorf("saved = %+v, want none", tc.repo.saved)
				}
			} else if len(tc.repo.saved) != 1 || tc.repo.saved[0].StopBlocks != tc.wantSaved {
				t.Errorf("saved = %+v, want one run with StopBlocks %d", tc.repo.saved, tc.wantSaved)
			}
			ended := spans.Ended()
			if len(ended) != 1 || ended[0].Name() != spanName {
				t.Fatalf("spans = %+v, want one %s", ended, spanName)
			}
			for _, a := range ended[0].Attributes() {
				if string(a.Key) == attrBlock && a.Value.AsBool() == tc.wantBlock {
					return
				}
			}
			t.Errorf("span attributes = %v, want %s=%v", ended[0].Attributes(), attrBlock, tc.wantBlock)
		})
	}
}

func TestDecideStopRepositoryErrors(t *testing.T) {
	errStore := errors.New("store down")
	cases := []struct {
		name string
		repo *fakeRepository
	}{
		{"load error", &fakeRepository{activeErr: errStore}},
		{"save error", &fakeRepository{stored: storedRun(domain.PhaseWorking, 0), saveErr: errStore}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spans := tracetest.NewSpanRecorder()
			tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test")
			handler := decidestop.NewHandler(tc.repo, domain.Policy{MaxStopBlocks: maxStopBlocks}, tracer)

			result, err := handler.Handle(context.Background(), decidestop.Command{Key: key})

			if !errors.Is(err, errStore) || result.Block {
				t.Fatalf("result = %+v, err = %v, want no block and %v", result, err, errStore)
			}
			if ended := spans.Ended(); len(ended) != 1 || ended[0].Status().Code != codes.Error {
				t.Fatalf("spans = %+v, want one span with an error status", ended)
			}
		})
	}
}
