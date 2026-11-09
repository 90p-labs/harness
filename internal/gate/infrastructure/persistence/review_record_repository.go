package persistence

import (
	"context"

	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	selectReviewRecordModels = "SELECT DISTINCT model FROM review_records WHERE repo = ? AND head_sha = ? ORDER BY model"
	upsertReviewRecord       = "INSERT INTO review_records (repo, head_sha, model, at) VALUES (?, ?, ?, ?)" +
		" ON CONFLICT (repo, head_sha, model) DO UPDATE SET at = excluded.at"
)

type SQLReviewRecordRepository struct {
	db *dbal.DB
}

func NewSQLReviewRecordRepository(db *dbal.DB) SQLReviewRecordRepository {
	return SQLReviewRecordRepository{db: db}
}

func (r SQLReviewRecordRepository) Save(ctx context.Context, record domain.ReviewRecord) error {
	_, err := r.db.ExecContext(ctx, upsertReviewRecord, record.Repo, record.HeadSHA, record.Model, record.At.UTC())
	return err
}

func (r SQLReviewRecordRepository) Models(ctx context.Context, repo, headSHA string) ([]string, error) {
	rows, err := r.db.QueryContext(ctx, selectReviewRecordModels, repo, headSHA)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var models []string
	for rows.Next() {
		var model string
		if err := rows.Scan(&model); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}
