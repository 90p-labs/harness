package transition_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jorge07/harness/internal/run/application/command/transition"
	"github.com/jorge07/harness/internal/run/domain"
)

const (
	callActive = "active"
	callBefore = "before"
	callSave   = "save"
	callAfter  = "after"

	spanName      = "run.transition"
	attrRunID     = "harness.run.id"
	attrEvent     = "harness.event"
	attrPhaseFrom = "harness.phase.from"
	attrPhaseTo   = "harness.phase.to"

	runID     = "20261108-011805-add-login"
	vetoText  = "tests are red"
	startedAt = 4
)

var (
	now = time.Date(2026, 11, 8, 1, 18, 5, 0, time.UTC)
	key = domain.RunKey{Repo: "github.com/jorge07/harness", Worktree: "/work/harness"}
)

type journal struct{ calls []string }

type fakeRepository struct {
	journal *journal
	stored  domain.Run
	saved   []domain.Run
	saveErr error
}

func (f *fakeRepository) Active(context.Context, domain.RunKey) (domain.Run, error) {
	f.journal.calls = append(f.journal.calls, callActive)
	return f.stored, nil
}

func (f *fakeRepository) Get(context.Context, string) (domain.Run, error) { return f.stored, nil }

func (f *fakeRepository) Save(_ context.Context, run domain.Run) (domain.Run, error) {
	f.journal.calls = append(f.journal.calls, callSave)
	if f.saveErr != nil {
		return domain.Run{}, f.saveErr
	}
	f.saved = append(f.saved, run)
	return run.Persisted(), nil
}

type fakeHooks struct {
	journal   *journal
	veto      error
	beforeCmd domain.Command
	afterFrom domain.Run
	afterTo   domain.Run
}

func (f *fakeHooks) Before(_ context.Context, _ domain.Run, cmd domain.Command) error {
	f.journal.calls = append(f.journal.calls, callBefore)
	f.beforeCmd = cmd
	return f.veto
}

func (f *fakeHooks) After(_ context.Context, from domain.Run, to domain.Run) {
	f.journal.calls = append(f.journal.calls, callAfter)
	f.afterFrom, f.afterTo = from, to
}

type fixture struct {
	journal *journal
	repo    *fakeRepository
	hooks   *fakeHooks
	spans   *tracetest.SpanRecorder
	handler transition.Handler
}

func newFixture(stored domain.Run) fixture {
	j := &journal{}
	repo := &fakeRepository{journal: j, stored: stored}
	hooks := &fakeHooks{journal: j}
	spans := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test")
	flow := transition.NewFlow(repo, hooks, domain.Policy{MaxReviewRounds: 3}, func() time.Time { return now }, tracer)
	return fixture{journal: j, repo: repo, hooks: hooks, spans: spans, handler: transition.NewHandler(repo, flow)}
}

func workingRun() domain.Run {
	return domain.Run{ID: runID, Key: key, Phase: domain.PhaseWorking, Version: startedAt}
}

func submitForReview() transition.Command {
	return transition.Command{Key: key, Event: domain.EventSubmitForReview}
}

func assertCalls(t *testing.T, f fixture, want ...string) {
	t.Helper()
	if !reflect.DeepEqual(f.journal.calls, want) {
		t.Fatalf("calls = %v, want %v", f.journal.calls, want)
	}
}

func TestTransitionOrder(t *testing.T) {
	f := newFixture(workingRun())

	if err := f.handler.Handle(context.Background(), submitForReview()); err != nil {
		t.Fatal(err)
	}

	assertCalls(t, f, callActive, callBefore, callSave, callAfter)
	if !f.hooks.beforeCmd.At.Equal(now) {
		t.Errorf("Before At = %v, want %v", f.hooks.beforeCmd.At, now)
	}
	if f.hooks.afterFrom.Phase != domain.PhaseWorking || f.hooks.afterTo.Phase != domain.PhaseReviewing {
		t.Errorf("After got %s -> %s", f.hooks.afterFrom.Phase, f.hooks.afterTo.Phase)
	}
	if events := f.hooks.afterTo.Events(); len(events) != 1 || events[0].Event != domain.EventSubmitForReview {
		t.Errorf("After run events = %+v, want one %s", events, domain.EventSubmitForReview)
	}
	if len(f.repo.saved) != 1 || f.repo.saved[0].Phase != domain.PhaseReviewing {
		t.Fatalf("saved = %+v", f.repo.saved)
	}
	if got := f.repo.saved[0].Events()[0].At; !got.Equal(now) {
		t.Errorf("transition At = %v, want %v", got, now)
	}
}

func TestTransitionVetoed(t *testing.T) {
	f := newFixture(workingRun())
	f.hooks.veto = errors.Join(domain.ErrTransitionVetoed, errors.New(vetoText))

	err := f.handler.Handle(context.Background(), submitForReview())

	if !errors.Is(err, domain.ErrTransitionVetoed) || !strings.Contains(err.Error(), vetoText) {
		t.Fatalf("err = %v", err)
	}
	assertCalls(t, f, callActive, callBefore)
}

func TestTransitionIllegal(t *testing.T) {
	f := newFixture(workingRun())

	err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: domain.EventComplete})

	if !errors.Is(err, domain.ErrIllegalTransition) {
		t.Fatalf("err = %v", err)
	}
	for _, call := range f.journal.calls {
		if call == callSave || call == callAfter {
			t.Fatalf("calls = %v, want no %s", f.journal.calls, call)
		}
	}
}

func TestTransitionStale(t *testing.T) {
	f := newFixture(workingRun())
	f.repo.saveErr = domain.ErrStaleRun

	err := f.handler.Handle(context.Background(), submitForReview())

	if !errors.Is(err, domain.ErrStaleRun) {
		t.Fatalf("err = %v", err)
	}
	assertCalls(t, f, callActive, callBefore, callSave)
}

func TestTransitionSpan(t *testing.T) {
	t.Run("recorded with both phases", func(t *testing.T) {
		f := newFixture(workingRun())

		if err := f.handler.Handle(context.Background(), submitForReview()); err != nil {
			t.Fatal(err)
		}

		ended := f.spans.Ended()
		if len(ended) != 1 {
			t.Fatalf("spans = %d, want 1", len(ended))
		}
		if ended[0].Name() != spanName || ended[0].Status().Code != codes.Unset {
			t.Errorf("span %q status %v", ended[0].Name(), ended[0].Status())
		}
		want := map[string]string{
			attrRunID:     runID,
			attrEvent:     string(domain.EventSubmitForReview),
			attrPhaseFrom: string(domain.PhaseWorking),
			attrPhaseTo:   string(domain.PhaseReviewing),
		}
		if got := stringAttributes(ended[0]); !reflect.DeepEqual(got, want) {
			t.Errorf("attributes = %v, want %v", got, want)
		}
	})

	t.Run("a veto sets the status to error", func(t *testing.T) {
		f := newFixture(workingRun())
		f.hooks.veto = domain.ErrTransitionVetoed

		if err := f.handler.Handle(context.Background(), submitForReview()); err == nil {
			t.Fatal("want a veto")
		}

		ended := f.spans.Ended()
		if len(ended) != 1 || ended[0].Status().Code != codes.Error {
			t.Fatalf("spans = %v", ended)
		}
		if _, ok := stringAttributes(ended[0])[attrPhaseTo]; ok {
			t.Errorf("a vetoed span has %s", attrPhaseTo)
		}
	})
}

func stringAttributes(span sdktrace.ReadOnlySpan) map[string]string {
	out := map[string]string{}
	for _, kv := range span.Attributes() {
		out[string(kv.Key)] = kv.Value.AsString()
	}
	return out
}
