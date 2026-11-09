package persistence_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/gate/infrastructure/persistence"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	repoName  = "github.com/90p-labs/harness"
	gateID    = "go-test"
	inputHash = "9f2c1a"
	headSHA   = "a1b2c3d"
	baseRef   = "main"
)

var clock = time.Date(2026, 11, 8, 12, 30, 0, 0, time.FixedZone("CET", 3600))

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

func evidence(result domain.Result, duration time.Duration, tail string) domain.Evidence {
	return domain.Evidence{Repo: repoName, GateID: gateID, InputHash: inputHash, Result: result, Duration: duration, Tail: tail, At: clock}
}

func TestEvidenceRepositoryContract(t *testing.T) {
	ctx := context.Background()

	t.Run("commit checks", testCommitCheckContract)

	t.Run("saves and finds evidence", func(t *testing.T) {
		repo := persistence.NewSQLEvidenceRepository(openDB(t))
		want := evidence(domain.ResultPass, 1500*time.Millisecond, "ok\n")
		if err := repo.Save(ctx, want); err != nil {
			t.Fatal(err)
		}
		got, found, err := repo.Find(ctx, repoName, gateID, inputHash)
		if err != nil || !found {
			t.Fatalf("found=%v err=%v", found, err)
		}
		if got.Result != want.Result || got.Duration != want.Duration || got.Tail != want.Tail || !got.At.Equal(want.At) ||
			got.Repo != want.Repo || got.GateID != want.GateID || got.InputHash != want.InputHash {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("a missing key is not found", func(t *testing.T) {
		repo := persistence.NewSQLEvidenceRepository(openDB(t))
		if err := repo.Save(ctx, evidence(domain.ResultPass, time.Second, "")); err != nil {
			t.Fatal(err)
		}
		for name, key := range map[string][3]string{
			"hash": {repoName, gateID, "other"},
			"gate": {repoName, "lint", inputHash},
			"repo": {"other/repo", gateID, inputHash},
		} {
			_, found, err := repo.Find(ctx, key[0], key[1], key[2])
			if err != nil || found {
				t.Fatalf("%s: found=%v err=%v", name, found, err)
			}
		}
	})

	t.Run("saving the same key replaces the record", func(t *testing.T) {
		db := openDB(t)
		repo := persistence.NewSQLEvidenceRepository(db)
		if err := repo.Save(ctx, evidence(domain.ResultFail, time.Second, "boom")); err != nil {
			t.Fatal(err)
		}
		if err := repo.Save(ctx, evidence(domain.ResultPass, 2*time.Second, "ok")); err != nil {
			t.Fatal(err)
		}
		got, found, err := repo.Find(ctx, repoName, gateID, inputHash)
		if err != nil || !found {
			t.Fatalf("found=%v err=%v", found, err)
		}
		if got.Result != domain.ResultPass || got.Duration != 2*time.Second || got.Tail != "ok" {
			t.Fatalf("got %+v", got)
		}
		var rows int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM gate_evidence").Scan(&rows); err != nil || rows != 1 {
			t.Fatalf("rows=%d err=%v", rows, err)
		}
	})
}

func testCommitCheckContract(t *testing.T) {
	ctx := context.Background()
	check := func(passed bool) domain.CommitCheck {
		return domain.CommitCheck{Repo: repoName, HeadSHA: headSHA, Base: baseRef, Passed: passed, At: clock}
	}

	t.Run("a passed check is true", func(t *testing.T) {
		repo := persistence.NewSQLCommitCheckRepository(openDB(t))
		if err := repo.Save(ctx, check(true)); err != nil {
			t.Fatal(err)
		}
		passed, err := repo.Passed(ctx, repoName, headSHA)
		if err != nil || !passed {
			t.Fatalf("passed=%v err=%v", passed, err)
		}
	})

	t.Run("a missing check is false", func(t *testing.T) {
		repo := persistence.NewSQLCommitCheckRepository(openDB(t))
		if err := repo.Save(ctx, check(true)); err != nil {
			t.Fatal(err)
		}
		for name, key := range map[string][2]string{"head": {repoName, "ffffff"}, "repo": {"other/repo", headSHA}} {
			passed, err := repo.Passed(ctx, key[0], key[1])
			if err != nil || passed {
				t.Fatalf("%s: passed=%v err=%v", name, passed, err)
			}
		}
	})

	t.Run("a failed check is false, and a later pass replaces it", func(t *testing.T) {
		repo := persistence.NewSQLCommitCheckRepository(openDB(t))
		if err := repo.Save(ctx, check(false)); err != nil {
			t.Fatal(err)
		}
		passed, err := repo.Passed(ctx, repoName, headSHA)
		if err != nil || passed {
			t.Fatalf("failed: passed=%v err=%v", passed, err)
		}
		if err := repo.Save(ctx, check(true)); err != nil {
			t.Fatal(err)
		}
		passed, err = repo.Passed(ctx, repoName, headSHA)
		if err != nil || !passed {
			t.Fatalf("replaced: passed=%v err=%v", passed, err)
		}
	})
}
