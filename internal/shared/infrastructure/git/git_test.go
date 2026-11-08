package git_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

func TestTopLevel(t *testing.T) {
	ctx := context.Background()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := git.TopLevel(ctx, sub)
	if err != nil || got != root {
		t.Fatalf("TopLevel = %q, %v, want %q", got, err, root)
	}

	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.TopLevel(ctx, outside); !errors.Is(err, git.ErrNotRepository) {
		t.Fatalf("outside a repo: %v, want ErrNotRepository", err)
	}
}

func TestTopLevelOtherGitError(t *testing.T) {
	bare := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", "--bare", bare).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	_, err := git.TopLevel(context.Background(), bare)

	if err == nil || errors.Is(err, git.ErrNotRepository) || !strings.Contains(err.Error(), "work tree") {
		t.Fatalf("bare repo: %v, want the git error", err)
	}
}
