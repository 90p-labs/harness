package submitplan_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/application/command/submitplan"
	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	validPlan   = "plan.md"
	brokenPlan  = "broken.md"
	docsPlan    = "docs.md"
	openPlan    = "open.md"
	problemLine = "plan.md:1: heading-missing: no ## Goal"
	testCwd     = "/work/tree"
	testRunID   = "20261108-011805-add-the-login-page"
	testPhase   = "planning"
	pluginName  = "advisor"
	denyReason  = "the plan misses a rollback"
	failureText = "exit status 2"
)

type fakeBus struct {
	result    shareddomain.DispatchResult
	envelopes []shareddomain.Envelope
}

func (f *fakeBus) Dispatch(_ context.Context, e shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	f.envelopes = append(f.envelopes, e)
	return f.result, nil
}

func fakeCheck(_ context.Context, path string, _ int) (domain.Report, error) {
	if path == brokenPlan {
		return domain.Report{Problems: []domain.Problem{{Line: 1, Rule: domain.RuleHeadingMissing, Message: "no ## Goal"}}}, nil
	}
	if path == docsPlan {
		doc := domain.Document{
			Commits:       []domain.Commit{{Subject: "docs: add the grill step to the harness skill"}},
			OpenQuestions: []string{domain.OpenQuestionsNone},
		}
		return domain.Report{Track: shareddomain.TrackLight, Doc: doc}, nil
	}
	if path == openPlan {
		return domain.Report{Track: shareddomain.TrackFull, Doc: domain.Document{OpenQuestions: []string{"Who owns the migration?"}}}, nil
	}
	return domain.Report{Track: shareddomain.TrackFull, Doc: domain.Document{OpenQuestions: []string{domain.OpenQuestionsNone}}}, nil
}

func newHandler(bus *fakeBus) submitplan.Handler {
	return submitplan.NewHandler(fakeCheck, bus, testCwd)
}

func command(path string) submitplan.Command {
	return submitplan.Command{RunID: testRunID, Phase: testPhase, Path: path}
}

const bugPath = "bug.md"

type fakeReproduction struct {
	record domain.ReproductionRecord
	found  bool
	calls  int
}

func (f *fakeReproduction) Reproduction(context.Context, int) (domain.ReproductionRecord, bool, error) {
	f.calls++
	return f.record, f.found, nil
}

func bugCheck(_ context.Context, _ string, _ int) (domain.Report, error) {
	doc := domain.Document{
		Cause:         domain.CauseBug,
		OpenQuestions: []string{domain.OpenQuestionsNone},
		Reproduction: domain.Reproduction{
			Line:      20,
			Criterion: domain.ReproLine{Value: "1", Line: 21},
			Symptom:   domain.ReproLine{Value: "`want pong, got ping`", Line: 22},
		},
	}
	return domain.Report{Track: shareddomain.TrackFull, Doc: doc}, nil
}

func TestReproductionRedSubmit(t *testing.T) {
	allow := shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}
	red := domain.ReproductionRecord{Tail: "want pong, got ping"}
	tests := []struct {
		name    string
		phase   string
		reader  *fakeReproduction
		refused bool
		calls   int
	}{
		{"planning with no record is refused", "planning", &fakeReproduction{}, true, 1},
		{"planning with a passed proof is refused", "planning", &fakeReproduction{record: domain.ReproductionRecord{Passed: true}, found: true}, true, 1},
		{"planning with a red proof is accepted", "planning", &fakeReproduction{record: red, found: true}, false, 1},
		{"another phase runs no red check", "awaiting_approval", &fakeReproduction{}, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			bus := &fakeBus{result: allow}
			handler := submitplan.NewHandler(bugCheck, bus, testCwd).WithReproduction(tc.reader)

			_, err := handler.Handle(t.Context(), submitplan.Command{RunID: testRunID, Phase: tc.phase, Path: bugPath})

			if tc.refused {
				if !errors.Is(err, domain.ErrPlanHasProblems) || !strings.Contains(err.Error(), string(domain.RuleReproductionRed)) {
					t.Fatalf("err = %v, want ErrPlanHasProblems with %s", err, domain.RuleReproductionRed)
				}
				if len(bus.envelopes) != 0 {
					t.Errorf("dispatches = %d, want 0", len(bus.envelopes))
				}
			} else if err != nil {
				t.Fatalf("Handle: %v", err)
			}
			if tc.reader.calls != tc.calls {
				t.Errorf("reader calls = %d, want %d", tc.reader.calls, tc.calls)
			}
		})
	}
	t.Run("a static problem skips the red check", func(t *testing.T) {
		reader := &fakeReproduction{}
		handler := newHandler(&fakeBus{}).WithReproduction(reader)

		_, err := handler.Handle(t.Context(), command(brokenPlan))

		if !errors.Is(err, domain.ErrPlanHasProblems) || reader.calls != 0 {
			t.Fatalf("err = %v, calls = %d, want the static problem and no call", err, reader.calls)
		}
	})
}

func TestSubmitPlanCollect(t *testing.T) {
	t.Run("allow returns the track and sends the envelope", func(t *testing.T) {
		bus := &fakeBus{result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}}

		result, err := newHandler(bus).Handle(t.Context(), command(validPlan))

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if result != (submitplan.Result{Track: shareddomain.TrackFull}) {
			t.Errorf("result = %+v, want the track %q and no waiver", result, shareddomain.TrackFull)
		}
		if len(bus.envelopes) != 1 {
			t.Fatalf("dispatches = %d, want 1", len(bus.envelopes))
		}
		got := bus.envelopes[0]
		if got.Version != shareddomain.EnvelopeVersion || got.Event != shareddomain.EventPlanSubmitted || got.Cwd != testCwd {
			t.Errorf("envelope = %+v", got)
		}
		if got.Run != (shareddomain.RunRef{ID: testRunID, Phase: testPhase}) {
			t.Errorf("run = %+v", got.Run)
		}
		var payload map[string]string
		if err := json.Unmarshal(got.Payload, &payload); err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"path": validPlan, "track": string(shareddomain.TrackFull)}
		if payload["path"] != want["path"] || payload["track"] != want["track"] || len(payload) != len(want) {
			t.Errorf("payload = %v, want %v", payload, want)
		}
	})

	t.Run("deny stops the submit with the reason", func(t *testing.T) {
		bus := &fakeBus{result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: denyReason}}}

		result, err := newHandler(bus).Handle(t.Context(), command(validPlan))

		if !errors.Is(err, submitplan.ErrPlanRejected) || !strings.Contains(err.Error(), denyReason) {
			t.Fatalf("err = %v, want ErrPlanRejected with %q", err, denyReason)
		}
		if result != (submitplan.Result{}) {
			t.Errorf("result = %+v, want empty", result)
		}
	})

	t.Run("a docs-only plan with no open question waives the approval", func(t *testing.T) {
		bus := &fakeBus{result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}}

		result, err := newHandler(bus).Handle(t.Context(), command(docsPlan))

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if result != (submitplan.Result{Track: shareddomain.TrackLight, ApprovalWaived: true}) {
			t.Errorf("result = %+v, want the light track and the waiver", result)
		}
	})

	t.Run("a plan with an open question reports it", func(t *testing.T) {
		bus := &fakeBus{result: shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}}

		result, err := newHandler(bus).Handle(t.Context(), command(openPlan))

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if result != (submitplan.Result{Track: shareddomain.TrackFull, QuestionsOpen: true}) {
			t.Errorf("result = %+v, want the full track and the open questions", result)
		}
	})

	t.Run("a failure with an allow verdict stops the submit", func(t *testing.T) {
		bus := &fakeBus{result: shareddomain.DispatchResult{
			Verdict:  shareddomain.Verdict{Kind: shareddomain.VerdictAllow},
			Failures: []shareddomain.DispatchFailure{{Plugin: pluginName, Err: errors.New(failureText)}},
		}}

		_, err := newHandler(bus).Handle(t.Context(), command(validPlan))

		if !errors.Is(err, submitplan.ErrPlanRejected) || !strings.Contains(err.Error(), pluginName+": "+failureText) {
			t.Fatalf("err = %v, want ErrPlanRejected with the failure line", err)
		}
	})

	t.Run("problems stop the submit before any dispatch", func(t *testing.T) {
		bus := &fakeBus{}

		_, err := newHandler(bus).Handle(t.Context(), command(brokenPlan))

		if !errors.Is(err, domain.ErrPlanHasProblems) || !strings.Contains(err.Error(), problemLine) {
			t.Fatalf("err = %v, want ErrPlanHasProblems with the problem lines", err)
		}
		if len(bus.envelopes) != 0 {
			t.Errorf("dispatches = %d, want 0", len(bus.envelopes))
		}
	})
}

func TestProgramKind(t *testing.T) {
	bus := &fakeBus{}
	check := func(context.Context, string, int) (domain.Report, error) {
		return domain.Report{Program: &domain.Program{}}, nil
	}

	_, err := submitplan.NewHandler(check, bus, testCwd).Handle(t.Context(), command(validPlan))

	if !errors.Is(err, domain.ErrPlanHasProblems) || !strings.Contains(err.Error(), "plan.md:1: "+string(domain.RulePlanKind)+": ") {
		t.Fatalf("err = %v, want ErrPlanHasProblems with a %s line", err, domain.RulePlanKind)
	}
	if len(bus.envelopes) != 0 {
		t.Errorf("dispatches = %d, want 0", len(bus.envelopes))
	}
}
