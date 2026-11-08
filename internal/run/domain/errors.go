package domain

import "errors"

var (
	ErrIllegalTransition = errors.New("illegal transition")
	ErrRunNotFound       = errors.New("run not found")
	ErrStaleRun          = errors.New("stale run")
	ErrActiveRunExists   = errors.New("active run exists")
	ErrTransitionVetoed  = errors.New("transition vetoed")

	ErrApprovalNotRecorded = errors.New("no approval is recorded for this plan: ask the Plan question and let the user pick Approve")
	ErrOpenQuestions       = errors.New("the plan has open questions: grill the user until ## Open questions holds only - none")
)
