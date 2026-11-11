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

// guardSuffixBudget is the room the guard plugin needs for ". " plus
// StopHoldsTurnNextStep (34 bytes). The guard domain is not imported here,
// so the budget is hardcoded with margin: the host cap is 340 bytes
// (hostReasonCap in cmd/harness/guard_test.go), minus 40 for the suffix.
// Counts are in bytes, so a multibyte reason stays under the cap too.
const guardSuffixBudget = 300

func TestStopReasonIsShort(t *testing.T) {
	policy := domain.Policy{MaxStopBlocks: 2}
	long := strings.Repeat("x", 500)
	multibyte := strings.Repeat("é", 500)
	const sectionsProblem = `needs ## Next, ## Open findings, ## Unreviewed commits, ## Decisions, each with a "- " line`
	const clippedLong = "xxxxxxxxxxxx..."
	const clippedMultibyte = "éééééé..."
	for _, phase := range []domain.Phase{domain.PhasePlanning, domain.PhaseWorking, domain.PhaseReviewing, domain.PhaseShipping} {
		for _, header := range []domain.Phase{domain.PhaseAwaitingApproval, domain.PhaseReleasing, domain.PhaseObserving, domain.PhaseAbandoned, domain.Phase(long)} {
			run := domain.Run{Phase: phase, Step: 12345}
			bodies := map[string]string{
				"missing":        "",
				"over the limit": strings.Repeat("x", domain.HandoffLimit*100),
				"no phase":       "Step: 1\n",
				"wrong phase":    "Phase: " + string(header) + "\nStep: 1\n",
				"no step":        "Phase: " + string(phase) + "\n",
				"wrong step":     "Phase: " + string(phase) + "\nStep: " + long + "\n",
				"no section":     "Phase: " + string(phase) + "\nStep: 12345\n",
				"empty section":  "Phase: " + string(phase) + "\nStep: 12345\n## Next\n## Open findings\n- a\n## Unreviewed commits\n- a\n## Decisions\n- a\n",
				"no unreviewed":  "Phase: " + string(phase) + "\nStep: 12345\n## Next\n- a\n## Open findings\n- a\n## Unreviewed commits\n## Decisions\n- a\n",
			}
			// Each worst-case body must name every problem in its reason.
			every := map[string][]string{
				"worst case": {
					"phase " + clippedLong + ", want " + string(phase),
					"step " + clippedLong + ", want 12345",
					sectionsProblem,
				},
				"multibyte": {
					"phase " + clippedMultibyte + ", want " + string(phase),
					"step " + clippedMultibyte + ", want 12345",
					sectionsProblem,
				},
			}
			bodies["worst case"] = "Phase: " + long + "\nStep: " + long + "\n## Next\n## Open findings\n## Unreviewed commits\n"
			bodies["multibyte"] = "Phase: " + multibyte + "\nStep: " + multibyte + "\n## Next\n## Open findings\n## Unreviewed commits\n"
			for name, body := range bodies {
				t.Run(string(phase)+"/"+string(header)+"/"+name, func(t *testing.T) {
					var raw []byte
					if name != "missing" {
						raw = []byte(body)
					}
					refusal := domain.CheckHandoff(run, raw)
					if refusal == nil {
						t.Fatal("want a refusal")
					}
					current := run
					current.HandoffStep = current.Step
					verdict, _ := domain.DecideStop(current, policy, false, refusal)

					if len(verdict.Reason) > guardSuffixBudget {
						t.Errorf("reason has %d bytes, want at most %d: %q", len(verdict.Reason), guardSuffixBudget, verdict.Reason)
					}
					for _, want := range every[name] {
						if !strings.Contains(verdict.Reason, want) {
							t.Errorf("reason %q lacks the problem %q", verdict.Reason, want)
						}
					}
					for _, want := range []string{string(phase), "no background agent or shell task", "in the background"} {
						if !strings.Contains(verdict.Reason, want) {
							t.Errorf("reason %q lacks %q", verdict.Reason, want)
						}
					}
				})
			}
		}
	}
	t.Run("handoff written", func(t *testing.T) {
		run := domain.Run{Phase: domain.PhaseReviewing, Step: 1, HandoffStep: 1}
		verdict, _ := domain.DecideStop(run, policy, false, nil)
		if len(verdict.Reason) > guardSuffixBudget {
			t.Errorf("reason has %d bytes: %q", len(verdict.Reason), verdict.Reason)
		}
	})
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

func TestDecideStopWaitsOnPR(t *testing.T) {
	const url = "https://github.com/o/r/pull/1"
	policy := domain.Policy{MaxStopBlocks: 2}
	refusal := errors.New("handoff.md has 5000 bytes, the limit is 4096")

	cases := []struct {
		name        string
		run         domain.Run
		handoff     error
		wantBlock   bool
		wantHandoff bool
	}{
		{"shipping with a URL and a ready handoff allows", domain.Run{Phase: domain.PhaseShipping, PRURL: url, Step: 3, HandoffStep: 3, StopBlocks: 1}, nil, false, false},
		{"the first stop of a step still asks for the handoff", domain.Run{Phase: domain.PhaseShipping, PRURL: url, Step: 3, HandoffStep: 2}, nil, false, true},
		{"a handoff refusal blocks", domain.Run{Phase: domain.PhaseShipping, PRURL: url, Step: 3, HandoffStep: 3}, refusal, true, false},
		{"a missing handoff blocks", domain.Run{Phase: domain.PhaseShipping, PRURL: url, Step: 3, HandoffStep: 3}, domain.ErrHandoffMissing, true, false},
		{"shipping with no URL blocks", domain.Run{Phase: domain.PhaseShipping, Step: 3, HandoffStep: 3}, nil, true, false},
		{"working with a URL blocks", domain.Run{Phase: domain.PhaseWorking, PRURL: url, Step: 3, HandoffStep: 3}, nil, true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			verdict, next := domain.DecideStop(c.run, policy, false, c.handoff)
			if verdict.Block != c.wantBlock || verdict.Handoff != c.wantHandoff {
				t.Fatalf("got %+v", verdict)
			}
			wantBlocks, wantTotal := c.run.StopBlocks, c.run.StopBlocksTotal
			if c.wantBlock {
				wantBlocks++
				wantTotal++
			}
			if next.StopBlocks != wantBlocks || next.StopBlocksTotal != wantTotal {
				t.Fatalf("stop blocks %d total %d, want %d and %d", next.StopBlocks, next.StopBlocksTotal, wantBlocks, wantTotal)
			}
			if next.PRURL != c.run.PRURL || next.Phase != c.run.Phase {
				t.Fatalf("run changed: %+v", next)
			}
		})
	}
}
