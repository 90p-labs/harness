package findings_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/infrastructure/findings"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const runID = "20261110-000000-add-login"

func runIn(t *testing.T, content *string) domain.Run {
	t.Helper()
	worktree := t.TempDir()
	if content != nil {
		path := filepath.Join(worktree, shareddomain.RunFindingsFile(runID))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(*content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return domain.Run{ID: runID, Key: domain.RunKey{Worktree: worktree}}
}

func TestReworkNeedsFindings(t *testing.T) {
	good := "| ID | Severity | Verdict | Status |\n|---|---|---|---|\n| F1 | major | real | open |\n"
	bad := "| ID | Severity | Verdict | Status |\n|---|---|---|---|\n| F1 | huge | real | open |\n"

	t.Run("reads the file of the run folder", func(t *testing.T) {
		got, err := findings.File{}.Read(context.Background(), runIn(t, &good))
		if err != nil || len(got) != 1 || got[0].Severity != shareddomain.SeverityMajor {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("a missing file tells the lead to write it", func(t *testing.T) {
		_, err := findings.File{}.Read(context.Background(), runIn(t, nil))
		if err == nil || !strings.Contains(err.Error(), "write") || !strings.Contains(err.Error(), "findings.md") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("a missing file wraps ErrNoFindingsFile", func(t *testing.T) {
		_, err := findings.File{}.Read(context.Background(), runIn(t, nil))
		if !errors.Is(err, domain.ErrNoFindingsFile) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("a malformed file does not wrap ErrNoFindingsFile", func(t *testing.T) {
		_, err := findings.File{}.Read(context.Background(), runIn(t, &bad))
		if errors.Is(err, domain.ErrNoFindingsFile) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("a malformed file is an error", func(t *testing.T) {
		_, err := findings.File{}.Read(context.Background(), runIn(t, &bad))
		if err == nil || !strings.Contains(err.Error(), "line 3") {
			t.Fatalf("got %v", err)
		}
	})
}
