package domain

import "errors"

var (
	ErrIllegalTransition    = errors.New("illegal transition")
	ErrRunNotFound          = errors.New("run not found")
	ErrStaleRun             = errors.New("stale run")
	ErrActiveRunExists      = errors.New("active run exists")
	ErrTransitionVetoed     = errors.New("transition vetoed")
	ErrNoOpenFinding        = errors.New("findings.md has no real open finding: record the finding with Verdict real and Status open, or ship")
	ErrNoFindingsFile       = errors.New("findings file does not exist")
	ErrReasonNamesNoFinding = errors.New("the rework reason must name each open blocker and major finding by ID")

	ErrApprovalNotRecorded = errors.New("no approval is recorded for this plan: ask the Plan question and let the user pick Approve")
	ErrNotShipping         = errors.New("the run is not in shipping: wait on a pull request only after the review is approved and the run is shipping")
	ErrOpenQuestions       = errors.New("the plan has open questions: grill the user until ## Open questions holds only - none")
)
