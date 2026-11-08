package domain

type Guard struct {
	Name  string
	Holds func(run Run, policy Policy) bool
}

var (
	always = Guard{Name: "", Holds: func(Run, Policy) bool { return true }}

	approvalNeeded = Guard{Name: "approval needed", Holds: func(run Run, policy Policy) bool {
		return policy.ApprovalNeeded || run.LiveRound > 0
	}}
	noApprovalNeeded = Guard{Name: "no approval needed", Holds: func(run Run, policy Policy) bool {
		return !approvalNeeded.Holds(run, policy)
	}}

	roundsLeft = Guard{Name: "rounds left", Holds: func(run Run, policy Policy) bool {
		return run.ReviewRound+1 < policy.MaxReviewRounds
	}}
	lastRound = Guard{Name: "last round", Holds: func(run Run, policy Policy) bool {
		return !roundsLeft.Holds(run, policy)
	}}

	goLiveOn = Guard{Name: "go-live on", Holds: func(_ Run, policy Policy) bool {
		return policy.GoLive
	}}
	goLiveOff = Guard{Name: "go-live off", Holds: func(_ Run, policy Policy) bool {
		return !policy.GoLive
	}}

	liveRoundsLeft = Guard{Name: "live rounds left", Holds: func(run Run, policy Policy) bool {
		return run.LiveRound+1 < policy.MaxLiveRounds
	}}
	lastLiveRound = Guard{Name: "last live round", Holds: func(run Run, policy Policy) bool {
		return !liveRoundsLeft.Holds(run, policy)
	}}

	leftPlanning         = leftGuard(PhasePlanning)
	leftAwaitingApproval = leftGuard(PhaseAwaitingApproval)
	leftWorking          = leftGuard(PhaseWorking)
	leftReviewing        = leftGuard(PhaseReviewing)
	leftShipping         = leftGuard(PhaseShipping)
	leftReleasing        = leftGuard(PhaseReleasing)
	leftObserving        = leftGuard(PhaseObserving)
)

func leftGuard(phase Phase) Guard {
	return Guard{Name: "left " + string(phase), Holds: func(run Run, _ Policy) bool {
		return run.BlockedFrom == phase
	}}
}
