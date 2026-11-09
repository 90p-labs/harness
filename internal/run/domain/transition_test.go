package domain_test

import (
	"slices"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

var allPhases = []domain.Phase{
	domain.PhaseNone, domain.PhasePlanning, domain.PhaseAwaitingApproval, domain.PhaseWorking,
	domain.PhaseReviewing, domain.PhaseShipping, domain.PhaseReleasing, domain.PhaseObserving,
	domain.PhaseBlocked, domain.PhaseDone, domain.PhaseAbandoned,
}

var resumable = allPhases[1:8]

var allEvents = []domain.Event{
	domain.EventStart, domain.EventSubmitPlan, domain.EventApprovePlan, domain.EventAmend,
	domain.EventSubmitForReview, domain.EventRequestRework, domain.EventApproveReview,
	domain.EventComplete, domain.EventDeploy, domain.EventClose, domain.EventRegress,
	domain.EventBlock, domain.EventResume, domain.EventAbandon,
}

func TestTransitionsDeterministic(t *testing.T) {
	const maxRounds = 3
	table := domain.Transitions()
	for _, phase := range allPhases {
		for _, approval := range []bool{true, false} {
			for _, goLive := range []bool{true, false} {
				for review := 0; review <= maxRounds+1; review++ {
					for live := 0; live <= maxRounds+1; live++ {
						for _, blockedFrom := range allPhases {
							policy := domain.Policy{ApprovalNeeded: approval, GoLive: goLive, MaxReviewRounds: maxRounds, MaxLiveRounds: maxRounds}
							run := domain.Run{Phase: phase, ReviewRound: review, LiveRound: live, BlockedFrom: blockedFrom}
							for _, event := range allEvents {
								var targets []domain.Phase
								paired := false
								for _, row := range table {
									if row.From != phase || row.Event != event {
										continue
									}
									paired = true
									if row.Guard.Holds(run, policy) {
										targets = append(targets, row.To)
									}
								}
								if len(targets) > 1 {
									t.Fatalf("%+v %+v event %s: %d targets %v", run, policy, event, len(targets), targets)
								}
								resumeWithoutOrigin := event == domain.EventResume && !slices.Contains(resumable, blockedFrom)
								if paired && len(targets) == 0 && !resumeWithoutOrigin {
									t.Fatalf("%+v %+v event %s: guards cover no case", run, policy, event)
								}
							}
						}
					}
				}
			}
		}
	}
}

func TestTerminalPhases(t *testing.T) {
	for _, row := range domain.Transitions() {
		if row.From == domain.PhaseDone || row.From == domain.PhaseAbandoned {
			t.Errorf("terminal phase %s has the row %s", row.From, row.Event)
		}
	}
	for _, phase := range []domain.Phase{domain.PhaseDone, domain.PhaseAbandoned} {
		for _, event := range allEvents {
			_, err := domain.Run{Phase: phase}.Apply(domain.Command{Event: event}, domain.Policy{})
			if err == nil {
				t.Errorf("%s accepted %s", phase, event)
			}
		}
	}
}

func TestTransitionsReturnsCopy(t *testing.T) {
	first := domain.Transitions()
	first[0].To = domain.PhaseAbandoned
	if domain.Transitions()[0].To == domain.PhaseAbandoned {
		t.Fatal("Transitions exposes the table")
	}
}

func TestTransitionNamesMatchTable(t *testing.T) {
	inTable := map[string]bool{}
	for _, row := range domain.Transitions() {
		inTable[string(row.Event)] = true
		if !slices.Contains(shareddomain.Transitions, string(row.Event)) {
			t.Errorf("table event %q missing from shared Transitions", row.Event)
		}
	}
	for _, name := range shareddomain.Transitions {
		if !inTable[name] {
			t.Errorf("shared transition %q is not a table event", name)
		}
	}
}
