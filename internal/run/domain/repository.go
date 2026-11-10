package domain

import (
	"context"
	"time"
)

type RunKey struct {
	Repo     string
	Worktree string
}

type ClassCount struct {
	Class ReworkClass
	Count int
}

type TransitionLog interface {
	Transitions(ctx context.Context, runID string) ([]Transitioned, error)
	RepeatClasses(ctx context.Context, repo string) ([]ClassCount, error)
}

type ReworkLog interface {
	LastReworkAt(ctx context.Context, runID string) (time.Time, error)
	ReworkReasons(ctx context.Context, runID string) ([]string, error)
}

type RunRepository interface {
	Active(ctx context.Context, key RunKey) (Run, error)
	Get(ctx context.Context, id string) (Run, error)
	ListByPhase(ctx context.Context, repo string, phases []Phase) ([]Run, error)
	Save(ctx context.Context, run Run) (Run, error)
}
