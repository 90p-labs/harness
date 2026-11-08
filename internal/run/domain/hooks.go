package domain

import "context"

type TransitionHooks interface {
	Before(ctx context.Context, run Run, cmd Command) error
	After(ctx context.Context, from Run, to Run)
}
