package domain

import "context"

type LiveRun struct {
	ID        string
	Worktree  string
	Phase     Phase
	LiveRound int
	PRURL     string
	Version   int
}

type Runs interface {
	Live(ctx context.Context, repo string) ([]LiveRun, error)
	Apply(ctx context.Context, run LiveRun, event Event, reason string) error
	Signals(ctx context.Context, run LiveRun) ([]Threshold, error)
}

type RunFolder interface {
	Archive(run LiveRun, round int) error
}
