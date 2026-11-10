package persistence

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	selectApproval = "SELECT EXISTS (SELECT 1 FROM plan_approvals WHERE run_id = ? AND plan_sha256 = ?)"
	insertApproval = "INSERT INTO plan_approvals (run_id, plan_sha256, at) VALUES (?, ?, ?) ON CONFLICT (run_id, plan_sha256) DO UPDATE SET at = excluded.at"
	selectLatest   = "SELECT plan_sha256 FROM plan_approvals WHERE run_id = ? ORDER BY at DESC, rowid DESC LIMIT 1"
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

func (r SQLApprovalRepository) Latest(ctx context.Context, runID string) (string, bool, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, selectLatest, runID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return hash, err == nil, err
}
