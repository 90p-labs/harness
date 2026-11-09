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
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	callActive = "active"
	callGet    = "get"
	callBefore = "before"
	callSave   = "save"
	callAfter  = "after"
	callGate   = "gate"
	callHash   = "hash"
	callHas    = "has"

	spanName      = "run.transition"
	attrRunID     = "harness.run.id"
	attrEvent     = "harness.event"
	attrPhaseFrom = "harness.phase.from"
	attrPhaseTo   = "harness.phase.to"

	runID     = "20261108-011805-add-login"
	vetoText  = "tests are red"
	planHash  = "0fcc61abbe1c"
	byModel   = "fable"
	byReason  = "the user approved all plans"
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

func (f *fakeRepository) Get(context.Context, string) (domain.Run, error) {
	f.journal.calls = append(f.journal.calls, callGet)
	return f.stored, nil
}

func (f *fakeRepository) ListByPhase(context.Context, string, []domain.Phase) ([]domain.Run, error) {
	return nil, nil
}

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
	beforeTo  domain.Phase
	afterFrom domain.Run
	afterTo   domain.Run
}

func (f *fakeHooks) Before(_ context.Context, _ domain.Run, cmd domain.Command, to domain.Phase) error {
	f.journal.calls = append(f.journal.calls, callBefore)
	f.beforeCmd, f.beforeTo = cmd, to
	return f.veto
}

func (f *fakeHooks) After(_ context.Context, from domain.Run, to domain.Run) {
	f.journal.calls = append(f.journal.calls, callAfter)
	f.afterFrom, f.afterTo = from, to
}

type fakeGate struct {
	journal    *journal
	submission domain.Submission
	err        error
	runs       []domain.Run
}

func (f *fakeGate) Submit(_ context.Context, run domain.Run) (domain.Submission, error) {
	f.journal.calls = append(f.journal.calls, callGate)
	f.runs = append(f.runs, run)
	return f.submission, f.err
}

func (f *fakeGate) Hash(domain.Run) (string, error) {
	f.journal.calls = append(f.journal.calls, callHash)
	return planHash, nil
}

type fakeApprovals struct {
	journal  *journal
	recorded map[string]bool
}

func (f *fakeApprovals) Has(_ context.Context, id, hash string) (bool, error) {
	f.journal.calls = append(f.journal.calls, callHas)
	return f.recorded[id+hash], nil
}

func (f *fakeApprovals) Save(_ context.Context, id, hash string, _ time.Time) error {
	f.recorded[id+hash] = true
	return nil
}

type fixture struct {
	journal   *journal
	repo      *fakeRepository
	hooks     *fakeHooks
	gate      *fakeGate
	approvals *fakeApprovals
	spans     *tracetest.SpanRecorder
	handler   transition.Handler
}

func newFixture(stored domain.Run) fixture {
	return newFixtureWith(stored, domain.Policy{MaxReviewRounds: 3})
}

func newFixtureWith(stored domain.Run, policy domain.Policy) fixture {
	j := &journal{}
	repo := &fakeRepository{journal: j, stored: stored}
	hooks := &fakeHooks{journal: j}
	spans := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spans)).Tracer("test")
	gate := &fakeGate{journal: j, submission: domain.Submission{Track: shareddomain.TrackFull}}
	approvals := &fakeApprovals{journal: j, recorded: map[string]bool{}}
	flow := transition.NewFlow(repo, hooks, gate, approvals, policy, func() time.Time { return now }, tracer)
	return fixture{journal: j, repo: repo, hooks: hooks, gate: gate, approvals: approvals, spans: spans, handler: transition.NewHandler(repo, flow)}
}

func planningRun() domain.Run {
	return domain.Run{ID: runID, Key: key, Phase: domain.PhasePlanning, Version: startedAt}
}

func awaitingRun() domain.Run {
	return domain.Run{ID: runID, Key: key, Phase: domain.PhaseAwaitingApproval, Version: startedAt}
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

func TestTransitionByRunID(t *testing.T) {
	const url = "https://github.com/o/r/pull/9"
	shipping := domain.Run{ID: runID, Key: key, Phase: domain.PhaseShipping, Version: startedAt}
	complete := transition.Command{Event: domain.EventComplete, RunID: runID, Version: startedAt, PRURL: url}

	t.Run("the run is loaded by id and records the PR", func(t *testing.T) {
		f := newFixture(shipping)

		if err := f.handler.Handle(context.Background(), complete); err != nil {
			t.Fatal(err)
		}

		assertCalls(t, f, callGet, callBefore, callSave, callAfter)
		if f.hooks.beforeCmd.PRURL != url || len(f.repo.saved) != 1 || f.repo.saved[0].PRURL != url {
			t.Fatalf("before = %+v, saved = %+v", f.hooks.beforeCmd, f.repo.saved)
		}
	})

	t.Run("a moved version is stale", func(t *testing.T) {
		moved := shipping
		moved.Version++
		f := newFixture(moved)

		err := f.handler.Handle(context.Background(), complete)

		if !errors.Is(err, domain.ErrStaleRun) {
			t.Fatalf("err = %v, want %v", err, domain.ErrStaleRun)
		}
		assertCalls(t, f, callGet)
	})
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

func TestFlowPlanGate(t *testing.T) {
	errGate := errors.New("plan has problems")
	cases := []struct {
		name      string
		run       domain.Run
		event     domain.Event
		gateErr   error
		wantCalls []string
		wantErr   error
		wantSaved bool
	}{
		{"submit-plan calls the gate before Before and saves its track", planningRun(), domain.EventSubmitPlan, nil,
			[]string{callActive, callGate, callBefore, callSave, callAfter}, nil, true},
		{"amend calls the gate", workingRun(), domain.EventAmend, nil,
			[]string{callActive, callGate, callBefore, callSave, callAfter}, nil, true},
		{"submit-plan in the wrong phase skips the gate", workingRun(), domain.EventSubmitPlan, nil,
			[]string{callActive}, domain.ErrIllegalTransition, false},
		{"amend in the wrong phase skips the gate", planningRun(), domain.EventAmend, nil,
			[]string{callActive}, domain.ErrIllegalTransition, false},
		{"a gate error stops before Before", planningRun(), domain.EventSubmitPlan, errGate,
			[]string{callActive, callGate}, errGate, false},
		{"a non-plan event skips the gate", workingRun(), domain.EventSubmitForReview, nil,
			[]string{callActive, callBefore, callSave, callAfter}, nil, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(tc.run)
			f.gate.err = tc.gateErr

			err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: tc.event})

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			assertCalls(t, f, tc.wantCalls...)
			if saved := len(f.repo.saved) == 1; saved != tc.wantSaved {
				t.Fatalf("saved = %+v, want saved %v", f.repo.saved, tc.wantSaved)
			}
			if tc.event == domain.EventSubmitPlan && tc.wantSaved {
				if f.repo.saved[0].Track != shareddomain.TrackFull || f.hooks.beforeCmd.Track != shareddomain.TrackFull {
					t.Errorf("saved track = %q, Before track = %q, want %q", f.repo.saved[0].Track, f.hooks.beforeCmd.Track, shareddomain.TrackFull)
				}
				if len(f.gate.runs) != 1 || f.gate.runs[0].ID != runID {
					t.Errorf("gate runs = %+v, want the current run", f.gate.runs)
				}
			}
		})
	}
}

func TestApproveCommandNeedsTheRecord(t *testing.T) {
	approve := transition.Command{Key: key, Event: domain.EventApprovePlan}

	t.Run("no record refuses the approval", func(t *testing.T) {
		f := newFixture(awaitingRun())

		err := f.handler.Handle(context.Background(), approve)

		if !errors.Is(err, domain.ErrApprovalNotRecorded) {
			t.Fatalf("err = %v, want %v", err, domain.ErrApprovalNotRecorded)
		}
		assertCalls(t, f, callActive, callGate, callHash, callHas)
	})

	t.Run("a record for the plan hash approves the checked plan", func(t *testing.T) {
		f := newFixture(awaitingRun())
		f.approvals.recorded[runID+planHash] = true

		if err := f.handler.Handle(context.Background(), approve); err != nil {
			t.Fatal(err)
		}

		assertCalls(t, f, callActive, callGate, callHash, callHas, callBefore, callSave, callAfter)
		if got := f.repo.saved[0]; got.Phase != domain.PhaseWorking || got.Track != shareddomain.TrackFull {
			t.Fatalf("phase = %s, track = %q, want %s and %q", got.Phase, got.Track, domain.PhaseWorking, shareddomain.TrackFull)
		}
	})

	t.Run("a plan that fails the check is not approved", func(t *testing.T) {
		errGate := errors.New("plan has problems")
		f := newFixture(awaitingRun())
		f.approvals.recorded[runID+planHash] = true
		f.gate.err = errGate

		err := f.handler.Handle(context.Background(), approve)

		if !errors.Is(err, errGate) {
			t.Fatalf("err = %v, want %v", err, errGate)
		}
		assertCalls(t, f, callActive, callGate)
	})

	t.Run("a plan with open questions is not approved", func(t *testing.T) {
		f := newFixture(awaitingRun())
		f.approvals.recorded[runID+planHash] = true
		f.gate.submission.QuestionsOpen = true

		err := f.handler.Handle(context.Background(), approve)

		if !errors.Is(err, domain.ErrOpenQuestions) {
			t.Fatalf("err = %v, want %v", err, domain.ErrOpenQuestions)
		}
		assertCalls(t, f, callActive, callGate)
	})

	t.Run("approve in the wrong phase skips the record", func(t *testing.T) {
		f := newFixture(planningRun())

		err := f.handler.Handle(context.Background(), approve)

		if !errors.Is(err, domain.ErrIllegalTransition) {
			t.Fatalf("err = %v, want %v", err, domain.ErrIllegalTransition)
		}
		assertCalls(t, f, callActive)
	})
}

func TestApproveCommandByModel(t *testing.T) {
	f := newFixture(awaitingRun())

	err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: domain.EventApprovePlan, By: byModel, Reason: byReason})

	if err != nil {
		t.Fatal(err)
	}
	assertCalls(t, f, callActive, callGate, callBefore, callSave, callAfter)
	want := "approved by " + byModel + ": " + byReason
	if got := f.repo.saved[0].Events()[0].Reason; got != want {
		t.Fatalf("reason = %q, want %q", got, want)
	}
}

func TestWaiverSubmitPlan(t *testing.T) {
	approval := domain.Policy{ApprovalNeeded: true, MaxReviewRounds: 3}
	cases := []struct {
		name      string
		run       domain.Run
		event     domain.Event
		waived    bool
		wantPhase domain.Phase
	}{
		{"a waived plan skips the approval", planningRun(), domain.EventSubmitPlan, true, domain.PhaseWorking},
		{"a plan with no waiver waits for the approval", planningRun(), domain.EventSubmitPlan, false, domain.PhaseAwaitingApproval},
		{"an amend never takes the waiver", workingRun(), domain.EventAmend, true, domain.PhaseAwaitingApproval},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixtureWith(tc.run, approval)
			f.gate.submission.ApprovalWaived = tc.waived

			if err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: tc.event}); err != nil {
				t.Fatal(err)
			}

			if got := f.repo.saved[0].Phase; got != tc.wantPhase {
				t.Fatalf("phase = %s, want %s", got, tc.wantPhase)
			}
		})
	}
}

func TestBeforeSeesWaivedTarget(t *testing.T) {
	f := newFixtureWith(planningRun(), domain.Policy{ApprovalNeeded: true, MaxReviewRounds: 3})
	f.gate.submission.ApprovalWaived = true

	if err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: domain.EventSubmitPlan}); err != nil {
		t.Fatal(err)
	}

	if f.hooks.beforeTo != domain.PhaseWorking {
		t.Fatalf("before to = %s, want %s", f.hooks.beforeTo, domain.PhaseWorking)
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
