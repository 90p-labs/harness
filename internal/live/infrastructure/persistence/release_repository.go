package persistence

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"io/fs"
	"time"

	"github.com/90p-labs/harness/internal/live/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	Module        = "live"
	migrationsDir = "migrations"

	selectRelease = "SELECT pr_url, merge_sha, merged_at, deployed_at, environment FROM releases WHERE run_id = ?"
	upsertRelease = "INSERT INTO releases (run_id, pr_url, merge_sha, merged_at, deployed_at, environment) VALUES (?, ?, ?, ?, ?, ?)" +
		" ON CONFLICT (run_id) DO UPDATE SET pr_url = excluded.pr_url, merge_sha = excluded.merge_sha, merged_at = excluded.merged_at," +
		" deployed_at = excluded.deployed_at, environment = excluded.environment"
)

//go:embed migrations
var embedded embed.FS

var Migrations = func() fs.FS {
	sub, err := fs.Sub(embedded, migrationsDir)
	if err != nil {
		panic(err)
	}
	return sub
}()

type SQLReleaseRepository struct {
	db *dbal.DB
}

func NewSQLReleaseRepository(db *dbal.DB) SQLReleaseRepository {
	return SQLReleaseRepository{db: db}
}

func (r SQLReleaseRepository) Get(ctx context.Context, runID string) (domain.Release, error) {
	release := domain.Release{RunID: runID}
	var mergedAt, deployedAt string
	err := r.db.QueryRowContext(ctx, selectRelease, runID).Scan(&release.PRURL, &release.MergeSHA, &mergedAt, &deployedAt, &release.Environment)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Release{}, domain.ErrReleaseNotFound
	}
	if err != nil {
		return domain.Release{}, err
	}
	if release.MergedAt, err = parseTime(mergedAt); err != nil {
		return domain.Release{}, err
	}
	if release.DeployedAt, err = parseTime(deployedAt); err != nil {
		return domain.Release{}, err
	}
	return release, nil
}

func (r SQLReleaseRepository) Save(ctx context.Context, release domain.Release) error {
	_, err := r.db.ExecContext(ctx, upsertRelease, release.RunID, release.PRURL, release.MergeSHA,
		formatTime(release.MergedAt), formatTime(release.DeployedAt), release.Environment)
	return err
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func parseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	return time.Parse(time.RFC3339Nano, s)
}
