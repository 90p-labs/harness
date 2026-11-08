package archive_test

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/jorge07/harness/internal/live/domain"
	"github.com/jorge07/harness/internal/live/infrastructure/archive"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	runID      = "20261108-072825-live"
	planBody   = "# Plan\n"
	reviewBody = "# Review\n"
	oldBody    = "old round\n"
)

func runFolder(t *testing.T) (domain.LiveRun, string) {
	t.Helper()
	worktree := t.TempDir()
	folder := filepath.Join(worktree, filepath.FromSlash(path.Dir(shareddomain.RunPlanFile(runID))))
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	return domain.LiveRun{ID: runID, Worktree: worktree}, folder
}

func write(t *testing.T, file, body string) {
	t.Helper()
	if err := os.WriteFile(file, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, file string) string {
	t.Helper()
	body, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestArchiveOnRegress(t *testing.T) {
	t.Run("copies both files", func(t *testing.T) {
		run, folder := runFolder(t)
		write(t, filepath.Join(folder, "plan.md"), planBody)
		write(t, filepath.Join(folder, "review.md"), reviewBody)

		if err := archive.New().Archive(run, 2); err != nil {
			t.Fatal(err)
		}

		if got := read(t, filepath.Join(folder, "plan-2.md")); got != planBody {
			t.Fatalf("plan-2.md = %q", got)
		}
		if got := read(t, filepath.Join(folder, "review-2.md")); got != reviewBody {
			t.Fatalf("review-2.md = %q", got)
		}
		if got := read(t, filepath.Join(folder, "plan.md")); got != planBody {
			t.Fatalf("plan.md = %q, the source must stay", got)
		}
	})

	t.Run("skips a missing review", func(t *testing.T) {
		run, folder := runFolder(t)
		write(t, filepath.Join(folder, "plan.md"), planBody)

		if err := archive.New().Archive(run, 1); err != nil {
			t.Fatal(err)
		}

		if _, err := os.Stat(filepath.Join(folder, "review-1.md")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("review-1.md stat error = %v, want not exist", err)
		}
		if got := read(t, filepath.Join(folder, "plan-1.md")); got != planBody {
			t.Fatalf("plan-1.md = %q", got)
		}
	})

	t.Run("refuses an existing target and writes nothing", func(t *testing.T) {
		run, folder := runFolder(t)
		write(t, filepath.Join(folder, "plan.md"), planBody)
		write(t, filepath.Join(folder, "review.md"), reviewBody)
		write(t, filepath.Join(folder, "review-3.md"), oldBody)

		err := archive.New().Archive(run, 3)

		if !errors.Is(err, domain.ErrArchiveExists) {
			t.Fatalf("err = %v, want ErrArchiveExists", err)
		}
		if got := read(t, filepath.Join(folder, "review-3.md")); got != oldBody {
			t.Fatalf("review-3.md = %q, want it untouched", got)
		}
		if _, err := os.Stat(filepath.Join(folder, "plan-3.md")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("plan-3.md stat error = %v, want not exist", err)
		}
	})

	t.Run("reports a read error that is not a missing file", func(t *testing.T) {
		run, folder := runFolder(t)
		if err := os.Mkdir(filepath.Join(folder, "plan.md"), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := archive.New().Archive(run, 1); err == nil {
			t.Fatal("want an error when plan.md is a directory")
		}
	})
}
