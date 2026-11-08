package domain

import shareddomain "github.com/jorge07/harness/internal/shared/domain"

type Event string

const (
	EventStart         Event = "start"
	EventSubmitPlan    Event = "submit-plan"
	EventApprovePlan   Event = "approve-plan"
	EventAmend         Event = "amend"
	EventRequestRework Event = "request-rework"
	EventApproveReview Event = "approve-review"
	EventComplete      Event = "complete"
	EventDeploy        Event = "deploy"
	EventClose         Event = "close"
	EventRegress       Event = "regress"
	EventBlock         Event = "block"
	EventResume        Event = "resume"
	EventAbandon       Event = "abandon"
)

const EventSubmitForReview = Event(shareddomain.TransitionSubmitForReview)
