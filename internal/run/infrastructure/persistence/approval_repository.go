package persistence

import (
	"context"
	"time"

	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	selectApproval = "SELECT EXISTS (SELECT 1 FROM plan_approvals WHERE run_id = ? AND plan_sha256 = ?)"
	insertApproval = "INSERT INTO plan_approvals (run_id, plan_sha256, at) VALUES (?, ?, ?) ON CONFLICT (run_id, plan_sha256) DO NOTHING"
)

type SQLApprovalRepository struct {
	db *dbal.DB
}

func NewSQLApprovalRepository(db *dbal.DB) SQLApprovalRepository {
	return SQLApprovalRepository{db: db}
}

func (r SQLApprovalRepository) Has(ctx context.Context, runID, planHash string) (bool, error) {
	var recorded bool
	err := r.db.QueryRowContext(ctx, selectApproval, runID, planHash).Scan(&recorded)
	return recorded, err
}

func (r SQLApprovalRepository) Save(ctx context.Context, runID, planHash string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, insertApproval, runID, planHash, at.UTC())
	return err
}
