package domain

import (
	"context"
	"path"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	runsFolder   = ".harness/runs"
	planFileName = "plan.md"
)

func PlanFile(runID string) string {
	return path.Join(runsFolder, runID, planFileName)
}

type PlanGate interface {
	Submit(ctx context.Context, run Run) (shareddomain.Track, error)
}

type TransitionHooks interface {
	Before(ctx context.Context, run Run, cmd Command) error
	After(ctx context.Context, from Run, to Run)
}
