package domain

import "context"

type PlanGate interface {
	Submit(ctx context.Context, run Run) (Submission, error)
	Hash(run Run) (string, error)
}

type TransitionHooks interface {
	Before(ctx context.Context, run Run, cmd Command) error
	After(ctx context.Context, from Run, to Run)
}
