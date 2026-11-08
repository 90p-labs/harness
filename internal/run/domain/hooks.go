package domain

import (
	"context"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type PlanGate interface {
	Submit(ctx context.Context, run Run) (shareddomain.Track, error)
}

type TransitionHooks interface {
	Before(ctx context.Context, run Run, cmd Command) error
	After(ctx context.Context, from Run, to Run)
}
