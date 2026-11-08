package domain

import "context"

type ReleaseRepository interface {
	Get(ctx context.Context, runID string) (Release, error)
	Save(ctx context.Context, release Release) error
}
