package domain

import (
	"context"
	"time"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type ApprovalRepository interface {
	Has(ctx context.Context, runID, planHash string) (bool, error)
	Save(ctx context.Context, runID, planHash string, at time.Time) error
}

type Submission struct {
	Track          shareddomain.Track
	ApprovalWaived bool
	QuestionsOpen  bool
}
