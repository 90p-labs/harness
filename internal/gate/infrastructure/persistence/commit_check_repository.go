package persistence

import (
	"context"
	"database/sql"
	"errors"

	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	selectCommitCheckPassed = "SELECT passed FROM commit_checks WHERE repo = ? AND head_sha = ?"
	upsertCommitCheck       = "INSERT INTO commit_checks (repo, head_sha, base, passed, at) VALUES (?, ?, ?, ?, ?)" +
		" ON CONFLICT (repo, head_sha) DO UPDATE SET base = excluded.base, passed = excluded.passed, at = excluded.at"
)

type SQLCommitCheckRepository struct {
	db *dbal.DB
}

func NewSQLCommitCheckRepository(db *dbal.DB) SQLCommitCheckRepository {
	return SQLCommitCheckRepository{db: db}
}

func (r SQLCommitCheckRepository) Passed(ctx context.Context, repo, headSHA string) (bool, error) {
	var passed bool
	err := r.db.QueryRowContext(ctx, selectCommitCheckPassed, repo, headSHA).Scan(&passed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return passed, err
}

func (r SQLCommitCheckRepository) Save(ctx context.Context, check domain.CommitCheck) error {
	_, err := r.db.ExecContext(ctx, upsertCommitCheck, check.Repo, check.HeadSHA, check.Base, check.Passed, check.At.UTC())
	return err
}
