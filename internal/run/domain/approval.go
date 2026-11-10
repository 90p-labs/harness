package domain

import (
	"context"
	"time"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type ApprovalRepository interface {
	Has(ctx context.Context, runID, planHash string) (bool, error)
	Save(ctx context.Context, runID, planHash string, at time.Time) error
	Latest(ctx context.Context, runID string) (hash string, ok bool, err error)
}

type Submission struct {
	Track          shareddomain.Track
	ApprovalWaived bool
	QuestionsOpen  bool
}
