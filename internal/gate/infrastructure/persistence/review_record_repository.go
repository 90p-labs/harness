package persistence

import (
	"context"
	"strings"
	"time"

	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	filesSeparator = "\n"

	selectReviewRecords = "SELECT head_sha, model, patch_id, files_read, run_id, at FROM review_records WHERE repo = ? AND " +
		"(head_sha = ? OR (patch_id <> '' AND patch_id = ?) OR (run_id <> '' AND run_id = ? AND at > ?)) ORDER BY model, head_sha"
	upsertReviewRecord = "INSERT INTO review_records (repo, head_sha, model, patch_id, files_read, run_id, at) VALUES (?, ?, ?, ?, ?, ?, ?)" +
		" ON CONFLICT (repo, head_sha, model) DO UPDATE SET patch_id = excluded.patch_id, files_read = excluded.files_read," +
		" run_id = excluded.run_id, at = excluded.at WHERE excluded.files_read <> '' OR review_records.files_read = ''"
)

type SQLReviewRecordRepository struct {
	db *dbal.DB
}

func NewSQLReviewRecordRepository(db *dbal.DB) SQLReviewRecordRepository {
	return SQLReviewRecordRepository{db: db}
}

func (r SQLReviewRecordRepository) Save(ctx context.Context, record domain.ReviewRecord) error {
	_, err := r.db.ExecContext(ctx, upsertReviewRecord, record.Repo, record.HeadSHA, record.Model, record.PatchID,
		strings.Join(record.FilesRead, filesSeparator), record.RunID, record.At.UTC())
	return err
}

func (r SQLReviewRecordRepository) Matching(ctx context.Context, scope domain.ReviewScope) ([]domain.ReviewRecord, error) {
	rows, err := r.db.QueryContext(ctx, selectReviewRecords, scope.Repo, scope.HeadSHA, scope.PatchID, scope.RunID, scope.RunSince.UTC())
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var records []domain.ReviewRecord
	for rows.Next() {
		record := domain.ReviewRecord{Repo: scope.Repo}
		var files string
		var at time.Time
		if err := rows.Scan(&record.HeadSHA, &record.Model, &record.PatchID, &files, &record.RunID, &at); err != nil {
			return nil, err
		}
		if files != "" {
			record.FilesRead = strings.Split(files, filesSeparator)
		}
		record.At = at.UTC()
		records = append(records, record)
	}
	return records, rows.Err()
}
