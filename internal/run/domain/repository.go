package domain

import "context"

type RunKey struct {
	Repo     string
	Worktree string
}

type RunRepository interface {
	Active(ctx context.Context, key RunKey) (Run, error)
	Get(ctx context.Context, id string) (Run, error)
	ListByPhase(ctx context.Context, repo string, phases []Phase) ([]Run, error)
	Save(ctx context.Context, run Run) (Run, error)
}
