package domain

import shareddomain "github.com/90p-labs/harness/internal/shared/domain"

type Phase string

const (
	PhaseNone             Phase = ""
	PhasePlanning         Phase = shareddomain.PhasePlanning
	PhaseAwaitingApproval Phase = shareddomain.PhaseAwaitingApproval
	PhaseWorking          Phase = shareddomain.PhaseWorking
	PhaseReviewing        Phase = shareddomain.PhaseReviewing
	PhaseShipping         Phase = shareddomain.PhaseShipping
	PhaseReleasing        Phase = shareddomain.PhaseReleasing
	PhaseObserving        Phase = shareddomain.PhaseObserving
	PhaseBlocked          Phase = shareddomain.PhaseBlocked
	PhaseDone             Phase = shareddomain.PhaseDone
	PhaseAbandoned        Phase = shareddomain.PhaseAbandoned
)

func (p Phase) HoldsTheTurn() bool {
	switch p {
	case PhasePlanning, PhaseWorking, PhaseReviewing, PhaseShipping:
		return true
	}
	return false
}
