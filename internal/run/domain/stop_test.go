package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
)

func TestDecideStop(t *testing.T) {
	policy := domain.Policy{MaxStopBlocks: 2}
	cases := []struct {
		name         string
		run          domain.Run
		agentRunning bool
		block        bool
	}{
		{"planning holds the turn", domain.Run{Phase: domain.PhasePlanning}, false, true},
		{"working holds the turn", domain.Run{Phase: domain.PhaseWorking}, false, true},
		{"reviewing holds the turn", domain.Run{Phase: domain.PhaseReviewing}, false, true},
		{"shipping holds the turn", domain.Run{Phase: domain.PhaseShipping}, false, true},
		{"agent running", domain.Run{Phase: domain.PhaseWorking}, true, false},
		{"limit reached", domain.Run{Phase: domain.PhaseWorking, StopBlocks: 2}, false, false},
		{"awaiting approval", domain.Run{Phase: domain.PhaseAwaitingApproval}, false, false},
		{"releasing", domain.Run{Phase: domain.PhaseReleasing}, false, false},
		{"observing", domain.Run{Phase: domain.PhaseObserving}, false, false},
		{"blocked", domain.Run{Phase: domain.PhaseBlocked}, false, false},
		{"done", domain.Run{Phase: domain.PhaseDone}, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			verdict, next := domain.DecideStop(c.run, policy, c.agentRunning, domain.ErrHandoffMissing)
			if verdict.Block != c.block {
				t.Fatalf("got %+v", verdict)
			}
			wantBlocks := c.run.StopBlocks
			if c.block {
				wantBlocks++
				if !strings.Contains(verdict.Reason, string(c.run.Phase)) {
					t.Fatalf("reason %q lacks the phase", verdict.Reason)
				}
				if !strings.Contains(verdict.Reason, "no background agent or shell task") || !strings.Contains(verdict.Reason, "in the background") {
					t.Fatalf("reason %q lacks the hook view or the legal wait", verdict.Reason)
				}
			}
			if next.StopBlocks != wantBlocks {
				t.Fatalf("stop blocks %d want %d", next.StopBlocks, wantBlocks)
			}
		})
	}
}

func TestStopBlocksTotal(t *testing.T) {
	policy := domain.Policy{MaxStopBlocks: 3}
	run := domain.Run{Phase: domain.PhaseWorking}
	for range 2 {
		_, run = domain.DecideStop(run, policy, false, domain.ErrHandoffMissing)
	}
	if run.StopBlocks != 2 || run.StopBlocksTotal != 2 {
		t.Fatalf("blocks = %d, total = %d, want 2 and 2", run.StopBlocks, run.StopBlocksTotal)
	}

	next, err := run.Apply(domain.Command{Event: domain.EventSubmitForReview}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if next.StopBlocks != 0 || next.StopBlocksTotal != 2 {
		t.Errorf("after Apply blocks = %d, total = %d, want 0 and 2", next.StopBlocks, next.StopBlocksTotal)
	}
}

func TestReworkClassIsRecorded(t *testing.T) {
	run := domain.Run{Phase: domain.PhaseReviewing}
	next, err := run.Apply(domain.Command{Event: domain.EventRequestRework, Reason: "F1", Class: domain.ReworkSpecGap}, domain.Policy{MaxReviewRounds: 3})
	if err != nil {
		t.Fatal(err)
	}
	if events := next.Events(); len(events) != 1 || events[0].Class != domain.ReworkSpecGap {
		t.Errorf("events = %+v, want class %s", events, domain.ReworkSpecGap)
	}
}

func TestDecideStopHandoff(t *testing.T) {
	policy := domain.Policy{MaxStopBlocks: 2}
	working := domain.Run{Phase: domain.PhaseWorking, Step: 3, HandoffStep: 2}
	refusal := errors.New("handoff.md has 5000 bytes, the limit is 4096")

	t.Run("ready handoff allows once", func(t *testing.T) {
		verdict, next := domain.DecideStop(working, policy, false, nil)
		if verdict.Block || !verdict.Handoff || next.HandoffStep != 3 || next.StopBlocks != 0 {
			t.Fatalf("got %+v, run %+v", verdict, next)
		}
		again, _ := domain.DecideStop(next, policy, false, nil)
		if !again.Block || again.Handoff {
			t.Fatalf("second stop got %+v", again)
		}
	})
	t.Run("agent running keeps the handoff step", func(t *testing.T) {
		verdict, next := domain.DecideStop(working, policy, true, nil)
		if verdict.Handoff || next.HandoffStep != 2 {
			t.Fatalf("got %+v, run %+v", verdict, next)
		}
	})
	t.Run("max blocks keeps the handoff step", func(t *testing.T) {
		run := working
		run.StopBlocks = 2
		verdict, next := domain.DecideStop(run, policy, false, nil)
		if verdict.Handoff || next.HandoffStep != 2 {
			t.Fatalf("got %+v, run %+v", verdict, next)
		}
	})
	t.Run("a refusal blocks and names the cause", func(t *testing.T) {
		verdict, next := domain.DecideStop(working, policy, false, refusal)
		if !verdict.Block || verdict.Handoff || !strings.Contains(verdict.Reason, refusal.Error()) || next.HandoffStep != 2 {
			t.Fatalf("got %+v, run %+v", verdict, next)
		}
	})
	t.Run("a missing handoff blocks and hints at the file", func(t *testing.T) {
		verdict, _ := domain.DecideStop(working, policy, false, domain.ErrHandoffMissing)
		if !verdict.Block || !strings.Contains(verdict.Reason, "handoff.md") {
			t.Fatalf("got %+v", verdict)
		}
	})
}
