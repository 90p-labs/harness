package domain_test

import (
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
			verdict, next := domain.DecideStop(c.run, policy, c.agentRunning)
			if verdict.Block != c.block {
				t.Fatalf("got %+v", verdict)
			}
			wantBlocks := c.run.StopBlocks
			if c.block {
				wantBlocks++
				if !strings.Contains(verdict.Reason, string(c.run.Phase)) {
					t.Fatalf("reason %q lacks the phase", verdict.Reason)
				}
			}
			if next.StopBlocks != wantBlocks {
				t.Fatalf("stop blocks %d want %d", next.StopBlocks, wantBlocks)
			}
		})
	}
}
