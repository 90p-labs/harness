package persistence

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	selectSetupRun = "SELECT input_hash FROM setup_runs WHERE repo = ? AND step_id = ?"
	upsertSetupRun = "INSERT INTO setup_runs (repo, step_id, input_hash, at) VALUES (?, ?, ?, ?)" +
		" ON CONFLICT (repo, step_id) DO UPDATE SET input_hash = excluded.input_hash, at = excluded.at"
)

type SQLSetupRepository struct {
	db *dbal.DB
}

func NewSQLSetupRepository(db *dbal.DB) SQLSetupRepository {
	return SQLSetupRepository{db: db}
}

func (r SQLSetupRepository) LastPass(ctx context.Context, repo, stepID string) (string, bool, error) {
	var hash string
	err := r.db.QueryRowContext(ctx, selectSetupRun, repo, stepID).Scan(&hash)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return hash, err == nil, err
}

func (r SQLSetupRepository) SavePass(ctx context.Context, repo, stepID, inputHash string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, upsertSetupRun, repo, stepID, inputHash, at.UTC())
	return err
}
