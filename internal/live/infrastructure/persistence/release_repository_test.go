package persistence_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/live/domain"
	"github.com/90p-labs/harness/internal/live/infrastructure/persistence"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	runID       = "20261108-072825-pr-9"
	otherRunID  = "20261108-090000-pr-10"
	prURL       = "https://github.com/90p-labs/harness/pull/9"
	mergeSHA    = "a1b2c3d"
	environment = "production"
)

var (
	mergedAt   = time.Date(2026, 11, 8, 12, 30, 15, 123456789, time.FixedZone("CET", 3600))
	deployedAt = time.Date(2026, 11, 8, 13, 5, 0, 0, time.UTC)
)

func openDB(t *testing.T) *dbal.DB {
	t.Helper()
	ctx := context.Background()
	db, err := dbal.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "state.db"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(ctx, persistence.Module, persistence.Migrations); err != nil {
		t.Fatal(err)
	}
	return db
}

func release(id string) domain.Release {
	return domain.Release{RunID: id, PRURL: prURL, MergeSHA: mergeSHA, MergedAt: mergedAt, DeployedAt: deployedAt, Environment: environment}
}

func assertSame(t *testing.T, got, want domain.Release) {
	t.Helper()
	if got.RunID != want.RunID || got.PRURL != want.PRURL || got.MergeSHA != want.MergeSHA ||
		got.Environment != want.Environment || !got.MergedAt.Equal(want.MergedAt) || !got.DeployedAt.Equal(want.DeployedAt) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestReleaseRepositoryContract(t *testing.T) {
	ctx := context.Background()

	t.Run("an unknown run is not found", func(t *testing.T) {
		repo := persistence.NewSQLReleaseRepository(openDB(t))
		if _, err := repo.Get(ctx, runID); !errors.Is(err, domain.ErrReleaseNotFound) {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("saves and gets a release", func(t *testing.T) {
		repo := persistence.NewSQLReleaseRepository(openDB(t))
		want := release(runID)
		if err := repo.Save(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Get(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		assertSame(t, got, want)
	})

	t.Run("saving the same run replaces the release", func(t *testing.T) {
		db := openDB(t)
		repo := persistence.NewSQLReleaseRepository(db)
		if err := repo.Save(ctx, release(runID)); err != nil {
			t.Fatal(err)
		}
		want := release(runID)
		want.MergeSHA = "ffeedd0"
		want.Environment = "staging"
		want.DeployedAt = deployedAt.Add(time.Hour)
		if err := repo.Save(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Get(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		assertSame(t, got, want)
		var rows int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM releases").Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("rows=%d err=%v", rows, err)
		}
	})

	t.Run("zero times round trip to zero", func(t *testing.T) {
		repo := persistence.NewSQLReleaseRepository(openDB(t))
		want := domain.Release{RunID: runID, PRURL: prURL}
		if err := repo.Save(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Get(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		if !got.MergedAt.IsZero() || !got.DeployedAt.IsZero() {
			t.Fatalf("got %+v", got)
		}
		assertSame(t, got, want)
	})

	t.Run("two runs are kept apart", func(t *testing.T) {
		repo := persistence.NewSQLReleaseRepository(openDB(t))
		other := release(otherRunID)
		other.PRURL = "https://github.com/90p-labs/harness/pull/10"
		if err := repo.Save(ctx, release(runID)); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, other); err != nil {
			t.Fatal(err)
		}
		first, err := repo.Get(ctx, runID)
		if err != nil {
			t.Fatal(err)
		}
		second, err := repo.Get(ctx, otherRunID)
		if err != nil {
			t.Fatal(err)
		}
		assertSame(t, first, release(runID))
		assertSame(t, second, other)
	})
}
