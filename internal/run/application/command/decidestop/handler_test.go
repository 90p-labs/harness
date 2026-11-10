package decidestop_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/run/application/command/decidestop"
	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	spanName  = "run.decide_stop"
	attrBlock = "harness.stop.block"

	runID         = "20261108-011805-add-login"
	maxStopBlocks = 3
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
			handler := decidestop.NewHandler(tc.repo, &fakeHandoff{}, domain.Policy{MaxStopBlocks: maxStopBlocks}, tracer)

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

type fakeHandoff struct {
	raw      []byte
	err      error
	worktree string
	runID    string
}

func (f *fakeHandoff) ReadHandoff(_ context.Context, worktree, runID string) ([]byte, error) {
	f.worktree, f.runID = worktree, runID
	return f.raw, f.err
}

func TestHandleHandoff(t *testing.T) {
	const step = 2
	ready := []byte("Phase: working\nStep: 2\n## Next\n- a\n## Open findings\n- none\n## Unreviewed commits\n- none\n## Decisions\n- none\n")
	errRead := errors.New("handoff.md is a symlink")
	cases := []struct {
		name      string
		reader    *fakeHandoff
		wantBlock bool
		wantStop  bool
		wantSaved int
	}{
		{"a ready file allows one stop and saves the handoff step", &fakeHandoff{raw: ready}, false, true, 1},
		{"a reader error blocks and keeps the block", &fakeHandoff{err: errRead}, true, false, 1},
		{"a missing file blocks", &fakeHandoff{}, true, false, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			run := storedRun(domain.PhaseWorking, 0)
			run.Step = step
			repo := &fakeRepository{stored: run}
			tracer := sdktrace.NewTracerProvider().Tracer("test")
			handler := decidestop.NewHandler(repo, tc.reader, domain.Policy{MaxStopBlocks: maxStopBlocks}, tracer)

			result, err := handler.Handle(context.Background(), decidestop.Command{Key: key})

			if err != nil {
				t.Fatal(err)
			}
			if result.Block != tc.wantBlock || result.Handoff != tc.wantStop {
				t.Fatalf("result = %+v, want block %v handoff %v", result, tc.wantBlock, tc.wantStop)
			}
			if tc.reader.runID != run.ID || tc.reader.worktree != run.Key.Worktree {
				t.Errorf("read handoff of run %q in %q, want run %q in %q", tc.reader.runID, tc.reader.worktree, run.ID, run.Key.Worktree)
			}
			if len(repo.saved) != tc.wantSaved {
				t.Fatalf("saved = %+v, want %d", repo.saved, tc.wantSaved)
			}
			if tc.wantStop && repo.saved[0].HandoffStep != step {
				t.Errorf("saved handoff step = %d, want %d", repo.saved[0].HandoffStep, step)
			}
			if tc.wantBlock && repo.saved[0].StopBlocks != 1 {
				t.Errorf("saved stop blocks = %d, want 1", repo.saved[0].StopBlocks)
			}
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
			handler := decidestop.NewHandler(tc.repo, &fakeHandoff{}, domain.Policy{MaxStopBlocks: maxStopBlocks}, tracer)

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
