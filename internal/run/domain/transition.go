package domain

import "slices"

type Transition struct {
	From  Phase
	Event Event
	Guard Guard
	To    Phase
}

var activePhases = []Phase{
	PhasePlanning, PhaseAwaitingApproval, PhaseWorking, PhaseReviewing,
	PhaseShipping, PhaseReleasing, PhaseObserving,
}

var table = buildTable()

func Transitions() []Transition {
	return slices.Clone(table)
}

func buildTable() []Transition {
	rows := []Transition{
		{PhaseNone, EventStart, always, PhasePlanning},
		{PhasePlanning, EventSubmitPlan, approvalNeeded, PhaseAwaitingApproval},
		{PhasePlanning, EventSubmitPlan, noApprovalNeeded, PhaseWorking},
		{PhaseAwaitingApproval, EventApprovePlan, always, PhaseWorking},
		{PhaseWorking, EventSubmitForReview, always, PhaseReviewing},
		{PhaseReviewing, EventRequestRework, roundsLeft, PhaseWorking},
		{PhaseReviewing, EventRequestRework, lastRound, PhaseBlocked},
		{PhaseReviewing, EventApproveReview, always, PhaseShipping},
		{PhaseShipping, EventComplete, goLiveOff, PhaseDone},
		{PhaseShipping, EventComplete, goLiveOn, PhaseReleasing},
		{PhaseReleasing, EventDeploy, always, PhaseObserving},
		{PhaseObserving, EventClose, always, PhaseDone},
		{PhaseBlocked, EventResume, leftPlanning, PhasePlanning},
		{PhaseBlocked, EventResume, leftAwaitingApproval, PhaseAwaitingApproval},
		{PhaseBlocked, EventResume, leftWorking, PhaseWorking},
		{PhaseBlocked, EventResume, leftReviewing, PhaseReviewing},
		{PhaseBlocked, EventResume, leftShipping, PhaseShipping},
		{PhaseBlocked, EventResume, leftReleasing, PhaseReleasing},
		{PhaseBlocked, EventResume, leftObserving, PhaseObserving},
		{PhaseBlocked, EventAbandon, always, PhaseAbandoned},
	}
	for _, phase := range []Phase{PhaseWorking, PhaseReviewing, PhaseShipping} {
		rows = append(rows,
			Transition{phase, EventAmend, approvalNeeded, PhaseAwaitingApproval},
			Transition{phase, EventAmend, noApprovalNeeded, PhaseWorking},
		)
	}
	for _, phase := range []Phase{PhaseReleasing, PhaseObserving} {
		rows = append(rows,
			Transition{phase, EventRegress, liveRoundsLeft, PhasePlanning},
			Transition{phase, EventRegress, lastLiveRound, PhaseBlocked},
		)
	}
	for _, phase := range activePhases {
		rows = append(rows,
			Transition{phase, EventBlock, always, PhaseBlocked},
			Transition{phase, EventAbandon, always, PhaseAbandoned},
		)
	}
	return rows
}
