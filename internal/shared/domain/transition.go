package domain

import "path"

const (
	TransitionStart           = "start"
	TransitionSubmitPlan      = "submit-plan"
	TransitionApprovePlan     = "approve-plan"
	TransitionAmend           = "amend"
	TransitionRequestRework   = "request-rework"
	TransitionReworkMinor     = "rework-minor"
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

	runsFolder        = ".harness/runs"
	planFileName      = "plan.md"
	findingsFileName  = "findings.md"
	handoffFileName   = "handoff.md"
	selfCheckFileName = "self-check.md"
)

var Phases = []string{
	PhasePlanning, PhaseAwaitingApproval, PhaseWorking, PhaseReviewing, PhaseShipping,
	PhaseReleasing, PhaseObserving, PhaseBlocked, PhaseDone, PhaseAbandoned,
}

var Transitions = []string{
	TransitionStart, TransitionSubmitPlan, TransitionApprovePlan, TransitionAmend,
	TransitionRequestRework, TransitionReworkMinor, TransitionSubmitForReview, TransitionApproveReview,
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

func RunFindingsFile(runID string) string {
	return path.Join(runsFolder, runID, findingsFileName)
}

func RunHandoffFile(runID string) string {
	return path.Join(runsFolder, runID, handoffFileName)
}

func RunSelfCheckFile(runID string) string {
	return path.Join(runsFolder, runID, selfCheckFileName)
}
