package transition_test

import (
	"context"
	"errors"
	"testing"

	"github.com/90p-labs/harness/internal/run/application/command/transition"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

var errNoFile = errors.New("findings.md missing")

func reviewingRun() domain.Run {
	return domain.Run{ID: runID, Key: key, Phase: domain.PhaseReviewing, Version: startedAt}
}

func realOpen(severity shareddomain.ReviewSeverity) shareddomain.ReviewFinding {
	return shareddomain.ReviewFinding{ID: "F1", Severity: severity, Verdict: shareddomain.FindingReal, Status: shareddomain.FindingOpen}
}

func rework() transition.Command {
	return transition.Command{Key: key, Event: domain.EventRequestRework, Reason: "fix F1"}
}

func TestReworkPicksTheEventFromTheFindings(t *testing.T) {
	cases := map[string]struct {
		findings  []shareddomain.ReviewFinding
		strict    bool
		wantEvent domain.Event
		wantRound int
	}{
		"a major counts":             {[]shareddomain.ReviewFinding{realOpen(shareddomain.SeverityMajor)}, false, domain.EventRequestRework, 1},
		"a minor keeps the round":    {[]shareddomain.ReviewFinding{realOpen(shareddomain.SeverityMinor)}, false, domain.EventReworkMinor, 0},
		"a minor counts when strict": {[]shareddomain.ReviewFinding{realOpen(shareddomain.SeverityMinor)}, true, domain.EventRequestRework, 1},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixtureWith(reviewingRun(), domain.Policy{MaxReviewRounds: 3, StrictMinor: tc.strict})
			f.findings.findings = tc.findings

			if err := f.handler.Handle(context.Background(), rework()); err != nil {
				t.Fatal(err)
			}

			assertCalls(t, f, callActive, callRead, callBefore, callSave, callAfter)
			saved := f.repo.saved[0]
			if f.hooks.beforeCmd.Event != tc.wantEvent || saved.ReviewRound != tc.wantRound || saved.Phase != domain.PhaseWorking {
				t.Fatalf("event %s, round %d, phase %s", f.hooks.beforeCmd.Event, saved.ReviewRound, saved.Phase)
			}
		})
	}
}

func TestReworkNeedsFindings(t *testing.T) {
	t.Run("a read error stops the rework", func(t *testing.T) {
		f := newFixture(reviewingRun())
		f.findings.err = errNoFile

		err := f.handler.Handle(context.Background(), rework())

		if !errors.Is(err, errNoFile) {
			t.Fatalf("got %v", err)
		}
		assertCalls(t, f, callActive, callRead)
	})

	t.Run("no real open finding stops the rework", func(t *testing.T) {
		f := newFixture(reviewingRun())

		err := f.handler.Handle(context.Background(), rework())

		if !errors.Is(err, domain.ErrNoOpenFinding) {
			t.Fatalf("got %v", err)
		}
		assertCalls(t, f, callActive, callRead)
	})

	t.Run("a reason that names no open finding stops the rework", func(t *testing.T) {
		f := newFixture(reviewingRun())
		f.findings.findings = []shareddomain.ReviewFinding{realOpen(shareddomain.SeverityMajor)}
		cmd := rework()
		cmd.Reason = "fix"

		err := f.handler.Handle(context.Background(), cmd)

		if !errors.Is(err, domain.ErrReasonNamesNoFinding) {
			t.Fatalf("got %v", err)
		}
		assertCalls(t, f, callActive, callRead)
	})

	t.Run("another event reads no findings", func(t *testing.T) {
		f := newFixture(workingRun())

		if err := f.handler.Handle(context.Background(), submitForReview()); err != nil {
			t.Fatal(err)
		}

		assertCalls(t, f, callActive, callBefore, callSave, callAfter)
	})
}
