package domain

import "path"

const (
	TransitionStart           = "start"
	TransitionSubmitPlan      = "submit-plan"
	TransitionApprovePlan     = "approve-plan"
	TransitionAmend           = "amend"
	TransitionRequestRework   = "request-rework"
	TransitionSubmitForReview = "submit-for-review"
	TransitionApproveReview   = "approve-review"
	TransitionComplete        = "complete"
	TransitionDeploy          = "deploy"
	TransitionClose           = "close"
	TransitionRegress         = "regress"
	TransitionBlock           = "block"
	TransitionResume          = "resume"
	TransitionAbandon         = "abandon"

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

var Phases = []string{
	PhasePlanning, PhaseAwaitingApproval, PhaseWorking, PhaseReviewing, PhaseShipping,
	PhaseReleasing, PhaseObserving, PhaseBlocked, PhaseDone, PhaseAbandoned,
}

var Transitions = []string{
	TransitionStart, TransitionSubmitPlan, TransitionApprovePlan, TransitionAmend,
	TransitionRequestRework, TransitionSubmitForReview, TransitionApproveReview,
	TransitionComplete, TransitionDeploy, TransitionClose, TransitionRegress,
	TransitionBlock, TransitionResume, TransitionAbandon,
}

type TransitionPayload struct {
	Event  string `json:"event"`
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

func RunPlanFile(runID string) string {
	return path.Join(runsFolder, runID, planFileName)
}
