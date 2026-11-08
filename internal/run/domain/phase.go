package domain

type Phase string

const (
	PhaseNone             Phase = ""
	PhasePlanning         Phase = "planning"
	PhaseAwaitingApproval Phase = "awaiting_approval"
	PhaseWorking          Phase = "working"
	PhaseReviewing        Phase = "reviewing"
	PhaseShipping         Phase = "shipping"
	PhaseReleasing        Phase = "releasing"
	PhaseObserving        Phase = "observing"
	PhaseBlocked          Phase = "blocked"
	PhaseDone             Phase = "done"
	PhaseAbandoned        Phase = "abandoned"
)

func (p Phase) HoldsTheTurn() bool {
	switch p {
	case PhasePlanning, PhaseWorking, PhaseReviewing, PhaseShipping:
		return true
	}
	return false
}
