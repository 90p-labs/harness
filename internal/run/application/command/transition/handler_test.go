package transition_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/run/application/command/transition"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
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
	callLatest = "latest"
	callKeep   = "keep"
	callTree   = "tree"
	callRow    = "row"

	spanName      = "run.transition"
	attrRunID     = "harness.run.id"
	attrEvent     = "harness.event"
	attrPhaseFrom = "harness.phase.from"
	attrPhaseTo   = "harness.phase.to"

	attrReviewRound    = "harness.review.round"
	attrReworkClass    = "harness.rework.class"
	attrReworkSeverity = "harness.rework.severity"

	runID     = "20261108-011805-add-login"
	vetoText  = "tests are red"
	planHash  = "0fcc61abbe1c"
	byModel   = "fable"
	byReason  = "the user approved all plans"
	startedAt = 4
)

var (
	now = time.Date(2026, 11, 8, 1, 18, 5, 0, time.UTC)
	key = domain.RunKey{Repo: "github.com/90p-labs/harness", Worktree: "/work/harness"}
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

type markerKey struct{}

type fakeTrace struct {
	journal    *journal
	under      []domain.Run
	ended      []domain.Run
	endedCtx   context.Context
	callsAtEnd []string
}

func (f *fakeTrace) Under(ctx context.Context, run domain.Run) context.Context {
	f.under = append(f.under, run)
	return context.WithValue(ctx, markerKey{}, true)
}

func (f *fakeTrace) End(ctx context.Context, run domain.Run) {
	f.callsAtEnd = slices.Clone(f.journal.calls)
	f.ended = append(f.ended, run)
	f.endedCtx = ctx
}

type fakeGate struct {
	journal     *journal
	submission  domain.Submission
	err         error
	runs        []domain.Run
	treeOnly    bool
	treeOnlyErr error
	treeHashes  []string
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

func (f *fakeGate) Keep(domain.Run) error {
	f.journal.calls = append(f.journal.calls, callKeep)
	return nil
}

func (f *fakeGate) TreeOnly(_ domain.Run, hash string) (bool, error) {
	f.journal.calls = append(f.journal.calls, callTree)
	f.treeHashes = append(f.treeHashes, hash)
	return f.treeOnly, f.treeOnlyErr
}

type fakeApprovals struct {
	journal   *journal
	recorded  map[string]bool
	saved     []string
	latest    string
	latestOK  bool
	latestErr error
}

func (f *fakeApprovals) Has(_ context.Context, id, hash string) (bool, error) {
	f.journal.calls = append(f.journal.calls, callHas)
	return f.recorded[id+hash], nil
}

func (f *fakeApprovals) Save(_ context.Context, id, hash string, _ time.Time) error {
	f.journal.calls = append(f.journal.calls, callRow)
	f.recorded[id+hash] = true
	f.saved = append(f.saved, hash)
	return nil
}

func (f *fakeApprovals) Latest(context.Context, string) (string, bool, error) {
	f.journal.calls = append(f.journal.calls, callLatest)
	return f.latest, f.latestOK, f.latestErr
}

type fixture struct {
	journal   *journal
	repo      *fakeRepository
	hooks     *fakeHooks
	gate      *fakeGate
	approvals *fakeApprovals
	trace     *fakeTrace
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
	runTrace := &fakeTrace{journal: j}
	flow := transition.NewFlow(repo, hooks, gate, approvals, policy, func() time.Time { return now }, tracer, runTrace)
	return fixture{journal: j, repo: repo, hooks: hooks, gate: gate, approvals: approvals, trace: runTrace, spans: spans, handler: transition.NewHandler(repo, flow)}
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

		assertCalls(t, f, callActive, callGate, callHash, callHas, callBefore, callSave, callRow, callKeep, callAfter)
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
	assertCalls(t, f, callActive, callGate, callHash, callBefore, callSave, callRow, callKeep, callAfter)
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
			attrRunID:       runID,
			attrEvent:       string(domain.EventSubmitForReview),
			attrPhaseFrom:   string(domain.PhaseWorking),
			attrPhaseTo:     string(domain.PhaseReviewing),
			attrReviewRound: "0",
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
		out[string(kv.Key)] = kv.Value.Emit()
	}
	return out
}

func TestTransitionSpanAttributes(t *testing.T) {
	reviewing := domain.Run{ID: runID, Key: key, Phase: domain.PhaseReviewing, ReviewRound: 1, Version: startedAt}
	rework := func(reason string, class domain.ReworkClass) transition.Command {
		return transition.Command{Key: key, Event: domain.EventRequestRework, Reason: reason, Class: class}
	}

	t.Run("every transition carries the review round", func(t *testing.T) {
		f := newFixture(workingRun())

		if err := f.handler.Handle(context.Background(), submitForReview()); err != nil {
			t.Fatal(err)
		}

		got := stringAttributes(f.spans.Ended()[0])
		if got[attrReviewRound] != "0" {
			t.Errorf("review round = %q, want 0", got[attrReviewRound])
		}
		if _, ok := got[attrReworkClass]; ok {
			t.Errorf("a non-rework span has %s", attrReworkClass)
		}
		if _, ok := got[attrReworkSeverity]; ok {
			t.Errorf("a non-rework span has %s", attrReworkSeverity)
		}
	})

	t.Run("a rework carries the round, the class and the highest severity but not the reason", func(t *testing.T) {
		const reason = "F1, major, a.go:12 secret detail; F2 blocker b.go:3"
		f := newFixture(reviewing)

		if err := f.handler.Handle(context.Background(), rework(reason, domain.ReworkSpecGap)); err != nil {
			t.Fatal(err)
		}

		got := stringAttributes(f.spans.Ended()[0])
		if got[attrReviewRound] != "2" || got[attrReworkClass] != string(domain.ReworkSpecGap) || got[attrReworkSeverity] != string(domain.SeverityBlocker) {
			t.Errorf("attributes = %v", got)
		}
		for key, value := range got {
			if strings.Contains(value, "secret detail") {
				t.Errorf("attribute %s holds the reason text", key)
			}
		}
	})

	t.Run("a rework without class or severity omits both", func(t *testing.T) {
		f := newFixture(reviewing)

		if err := f.handler.Handle(context.Background(), rework("broken", "")); err != nil {
			t.Fatal(err)
		}

		got := stringAttributes(f.spans.Ended()[0])
		if _, ok := got[attrReworkClass]; ok {
			t.Errorf("attributes = %v, want no class", got)
		}
		if _, ok := got[attrReworkSeverity]; ok {
			t.Errorf("attributes = %v, want no severity", got)
		}
	})
}

func TestMoveTracesTheRun(t *testing.T) {
	shipping := domain.Run{ID: runID, Key: key, Phase: domain.PhaseShipping, Version: startedAt}

	t.Run("a move that stays open puts the run under its trace and does not end it", func(t *testing.T) {
		f := newFixture(planningRun())

		if err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: domain.EventSubmitPlan}); err != nil {
			t.Fatal(err)
		}

		assertCalls(t, f, callActive, callGate, callBefore, callSave, callAfter)
		if len(f.trace.under) != 1 || f.trace.under[0].ID != runID || len(f.trace.ended) != 0 {
			t.Fatalf("under = %+v, ended = %+v", f.trace.under, f.trace.ended)
		}
	})

	cases := []struct {
		name  string
		run   domain.Run
		event domain.Event
		want  domain.Phase
	}{
		{"abandon ends the run", planningRun(), domain.EventAbandon, domain.PhaseAbandoned},
		{"complete ends the run", shipping, domain.EventComplete, domain.PhaseDone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(tc.run)

			if err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: tc.event}); err != nil {
				t.Fatal(err)
			}

			assertCalls(t, f, callActive, callBefore, callSave, callAfter)
			if !reflect.DeepEqual(f.trace.callsAtEnd, f.journal.calls) {
				t.Fatalf("End ran after %v, want after %v", f.trace.callsAtEnd, f.journal.calls)
			}
			if len(f.trace.ended) != 1 || f.trace.ended[0].Phase != tc.want || f.trace.ended[0].ID != runID {
				t.Fatalf("ended = %+v", f.trace.ended)
			}
			if f.trace.endedCtx.Value(markerKey{}) == nil {
				t.Fatal("End did not get the context from Under")
			}
		})
	}

	t.Run("a failed move does not end the run", func(t *testing.T) {
		f := newFixture(planningRun())
		f.repo.saveErr = errors.New("disk full")

		if err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: domain.EventAbandon}); err == nil {
			t.Fatal("want an error")
		}

		if len(f.trace.ended) != 0 {
			t.Fatalf("ended = %+v", f.trace.ended)
		}
	})
}

func TestApproveKeepsPlan(t *testing.T) {
	recorded := transition.Command{Key: key, Event: domain.EventApprovePlan}
	by := transition.Command{Key: key, Event: domain.EventApprovePlan, By: byModel, Reason: byReason}
	cases := []struct {
		name   string
		cmd    transition.Command
		veto   error
		stale  error
		record bool
		want   []string
	}{
		{"a recorded approval saves the hash and keeps the plan after the save", recorded, nil, nil, true,
			[]string{callActive, callGate, callHash, callHas, callBefore, callSave, callRow, callKeep, callAfter}},
		{"a by approval saves the hash and keeps the plan after the save", by, nil, nil, false,
			[]string{callActive, callGate, callHash, callBefore, callSave, callRow, callKeep, callAfter}},
		{"a veto of a recorded approval writes neither", recorded, errors.New(vetoText), nil, true,
			[]string{callActive, callGate, callHash, callHas, callBefore}},
		{"a veto of a by approval writes neither", by, errors.New(vetoText), nil, false,
			[]string{callActive, callGate, callHash, callBefore}},
		{"a stale save of a by approval writes neither", by, nil, domain.ErrStaleRun, false,
			[]string{callActive, callGate, callHash, callBefore, callSave}},
		{"a stale save of a recorded approval writes neither", recorded, nil, domain.ErrStaleRun, true,
			[]string{callActive, callGate, callHash, callHas, callBefore, callSave}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(awaitingRun())
			f.approvals.recorded[runID+planHash] = tc.record
			f.hooks.veto = tc.veto
			f.repo.saveErr = tc.stale

			_ = f.handler.Handle(context.Background(), tc.cmd)

			assertCalls(t, f, tc.want...)
			wroteHash := slices.Contains(tc.want, callRow)
			if wroteHash && !slices.Equal(f.approvals.saved, []string{planHash}) {
				t.Fatalf("saved hashes = %v, want [%s]", f.approvals.saved, planHash)
			}
		})
	}
}

func TestAmendTreeOnly(t *testing.T) {
	errRead := errors.New("read failed")
	const approvedHash = "approved-hash"
	reviewing := domain.Run{ID: runID, Key: key, Phase: domain.PhaseReviewing, Version: startedAt}
	shipping := domain.Run{ID: runID, Key: key, Phase: domain.PhaseShipping, Version: startedAt}
	live := workingRun()
	live.LiveRound = 1
	cases := []struct {
		name      string
		run       domain.Run
		latestOK  bool
		latestErr error
		treeOnly  bool
		treeErr   error
		wantPhase domain.Phase
		wantCalls []string
	}{
		{"a tree-only amend from working needs no approval", workingRun(), true, nil, true, nil, domain.PhaseWorking,
			[]string{callActive, callGate, callLatest, callTree, callBefore, callSave, callAfter}},
		{"a change outside the trees needs the approval", workingRun(), true, nil, false, nil, domain.PhaseAwaitingApproval,
			[]string{callActive, callGate, callLatest, callTree, callBefore, callSave, callAfter}},
		{"a live round needs the approval", live, true, nil, true, nil, domain.PhaseAwaitingApproval,
			[]string{callActive, callGate, callBefore, callSave, callAfter}},
		{"an amend from reviewing needs the approval", reviewing, true, nil, true, nil, domain.PhaseAwaitingApproval,
			[]string{callActive, callGate, callBefore, callSave, callAfter}},
		{"an amend from shipping needs the approval", shipping, true, nil, true, nil, domain.PhaseAwaitingApproval,
			[]string{callActive, callGate, callBefore, callSave, callAfter}},
		{"no latest row needs the approval", workingRun(), false, nil, true, nil, domain.PhaseAwaitingApproval,
			[]string{callActive, callGate, callLatest, callBefore, callSave, callAfter}},
		{"a latest error needs the approval", workingRun(), false, errRead, true, nil, domain.PhaseAwaitingApproval,
			[]string{callActive, callGate, callLatest, callBefore, callSave, callAfter}},
		{"a tree check error needs the approval", workingRun(), true, nil, true, errRead, domain.PhaseAwaitingApproval,
			[]string{callActive, callGate, callLatest, callTree, callBefore, callSave, callAfter}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixtureWith(tc.run, domain.Policy{ApprovalNeeded: true, MaxReviewRounds: 3})
			f.approvals.latest, f.approvals.latestOK, f.approvals.latestErr = approvedHash, tc.latestOK, tc.latestErr
			f.gate.treeOnly, f.gate.treeOnlyErr = tc.treeOnly, tc.treeErr

			if err := f.handler.Handle(context.Background(), transition.Command{Key: key, Event: domain.EventAmend}); err != nil {
				t.Fatal(err)
			}

			assertCalls(t, f, tc.wantCalls...)
			if got := f.repo.saved[0].Phase; got != tc.wantPhase {
				t.Fatalf("phase = %s, want %s", got, tc.wantPhase)
			}
			if slices.Contains(tc.wantCalls, callTree) && !slices.Equal(f.gate.treeHashes, []string{approvedHash}) {
				t.Fatalf("tree hashes = %v, want [%s]", f.gate.treeHashes, approvedHash)
			}
			if slices.Contains(tc.wantCalls, callKeep) {
				t.Fatal("a waived amend must not keep the plan")
			}
		})
	}
}
