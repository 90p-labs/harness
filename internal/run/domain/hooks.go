package domain

import "context"

type PlanGate interface {
	Submit(ctx context.Context, run Run) (Submission, error)
	Hash(run Run) (string, error)
	Keep(run Run) error
	StructureKept(run Run, approvedHash string) (bool, error)
}

type RunTrace interface {
	Under(ctx context.Context, run Run) context.Context
	End(ctx context.Context, run Run)
}

type TransitionHooks interface {
	Before(ctx context.Context, run Run, cmd Command, to Phase) error
	After(ctx context.Context, from Run, to Run)
}
