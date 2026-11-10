package domain

import shareddomain "github.com/90p-labs/harness/internal/shared/domain"

type Event string

const (
	EventStart           = Event(shareddomain.TransitionStart)
	EventSubmitPlan      = Event(shareddomain.TransitionSubmitPlan)
	EventApprovePlan     = Event(shareddomain.TransitionApprovePlan)
	EventAmend           = Event(shareddomain.TransitionAmend)
	EventRequestRework   = Event(shareddomain.TransitionRequestRework)
	EventReworkMinor     = Event(shareddomain.TransitionReworkMinor)
	EventSubmitForReview = Event(shareddomain.TransitionSubmitForReview)
	EventApproveReview   = Event(shareddomain.TransitionApproveReview)
	EventComplete        = Event(shareddomain.TransitionComplete)
	EventDeploy          = Event(shareddomain.TransitionDeploy)
	EventClose           = Event(shareddomain.TransitionClose)
	EventRegress         = Event(shareddomain.TransitionRegress)
	EventBlock           = Event(shareddomain.TransitionBlock)
	EventResume          = Event(shareddomain.TransitionResume)
	EventAbandon         = Event(shareddomain.TransitionAbandon)
)

func (e Event) IsRework() bool {
	return e == EventRequestRework || e == EventReworkMinor
}
