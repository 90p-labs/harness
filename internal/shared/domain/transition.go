package domain

import "path"

const (
	TransitionSubmitForReview = "submit-for-review"

	PhasePlanning         = "planning"
	PhaseAwaitingApproval = "awaiting_approval"
	PhaseWorking          = "working"
	PhaseReviewing        = "reviewing"
	PhaseShipping         = "shipping"
	PhaseReleasing        = "releasing"
	PhaseObserving        = "observing"
	PhaseBlocked          = "blocked"
	PhaseDone             = "done"
	PhaseAbandoned        = "abandoned"

	runsFolder   = ".harness/runs"
	planFileName = "plan.md"
)

type TransitionPayload struct {
	Event  string `json:"event"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

func RunPlanFile(runID string) string {
	return path.Join(runsFolder, runID, planFileName)
}
