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

func initRepo(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return root
}

func TestRepoKey(t *testing.T) {
	const github = "github.com/jorge07/harness"
	cases := map[string]struct{ url, want string }{
		"scp":             {"git@github.com:jorge07/harness.git", github},
		"https with user": {"https://user@github.com/jorge07/harness.git", github},
		"https":           {"https://github.com/jorge07/harness", github},
		"ssh with port":   {"ssh://git@github.com:22/jorge07/harness", github},
		"at sign in path": {"https://host.example/team/repo@v2.git", "host.example/team/repo@v2"},
		"scp at sign":     {"git@host.example:repo@v2.git", "host.example/repo@v2"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			root := initRepo(t)
			if out, err := exec.Command("git", "-C", root, "remote", "add", "origin", tc.url).CombinedOutput(); err != nil {
				t.Fatalf("git remote add: %v: %s", err, out)
			}
			sub := filepath.Join(root, "a")
			if err := os.MkdirAll(sub, 0o755); err != nil {
				t.Fatal(err)
			}

			repo, worktree, err := git.RepoKey(context.Background(), sub)

			if err != nil || repo != tc.want || worktree != root {
				t.Fatalf("RepoKey = %q, %q, %v, want %q, %q", repo, worktree, err, tc.want, root)
			}
		})
	}
}

func TestRepoKeyGitWithoutPathFormat(t *testing.T) {
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	shimDir := t.TempDir()
	shim := "#!/bin/sh\nfor arg do\n  shift\n  if [ \"$arg\" = --path-format=absolute ]; then echo \"$arg\"; else set -- \"$@\" \"$arg\"; fi\ndone\nexec " + realGit + " \"$@\"\n"
	if err := os.WriteFile(filepath.Join(shimDir, "git"), []byte(shim), 0o700); err != nil {
		t.Fatal(err)
	}
	root := initRepo(t)
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	repo, worktree, err := git.RepoKey(context.Background(), sub)

	wantCommon := filepath.Join(root, ".git")
	if err != nil || repo != wantCommon || worktree != root {
		t.Fatalf("RepoKey = %q, %q, %v, want %q, %q", repo, worktree, err, wantCommon, root)
	}
	common, err := git.CommonDir(context.Background(), sub)
	if err != nil || common != wantCommon {
		t.Fatalf("CommonDir = %q, %v, want %q", common, err, wantCommon)
	}
}

func TestRepoKeyNoOrigin(t *testing.T) {
	root := initRepo(t)

	repo, worktree, err := git.RepoKey(context.Background(), root)

	wantCommon := filepath.Join(root, ".git")
	if err != nil || repo != wantCommon || worktree != root {
		t.Fatalf("RepoKey = %q, %q, %v, want %q, %q", repo, worktree, err, wantCommon, root)
	}
	common, err := git.CommonDir(context.Background(), root)
	if err != nil || common != wantCommon {
		t.Fatalf("CommonDir = %q, %v, want %q", common, err, wantCommon)
	}
}

func TestRepoKeyOutsideRepository(t *testing.T) {
	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = git.RepoKey(context.Background(), outside)

	if !errors.Is(err, git.ErrNotRepository) {
		t.Fatalf("outside a repo: %v, want ErrNotRepository", err)
	}
}

func TestDefaultBranch(t *testing.T) {
	t.Run("origin HEAD set", func(t *testing.T) {
		root := initRepo(t)
		for _, args := range [][]string{
			{"commit", "-q", "--allow-empty", "-m", "init"},
			{"update-ref", "refs/remotes/origin/develop", "HEAD"},
			{"symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop"},
		} {
			if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v: %s", args, err, out)
			}
		}

		got, err := git.DefaultBranch(context.Background(), root)

		if err != nil || got != "develop" {
			t.Fatalf("DefaultBranch = %q, %v, want develop", got, err)
		}
	})

	t.Run("origin HEAD not set", func(t *testing.T) {
		got, err := git.DefaultBranch(context.Background(), initRepo(t))

		if err != nil || got != "main" {
			t.Fatalf("DefaultBranch = %q, %v, want main", got, err)
		}
	})

	t.Run("outside a repository", func(t *testing.T) {
		_, err := git.DefaultBranch(context.Background(), t.TempDir())

		if !errors.Is(err, git.ErrNotRepository) {
			t.Fatalf("outside a repo: %v, want ErrNotRepository", err)
		}
	})
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

func TestHead(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", root).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	if _, err := git.Head(ctx, root); !errors.Is(err, git.ErrNoHead) {
		t.Fatalf("Head without a commit: %v, want ErrNoHead", err)
	}

	if out, err := exec.Command("git", "-C", root, "commit", "-q", "--allow-empty", "-m", "first").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	want, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	got, err := git.Head(ctx, root)
	if err != nil || got != strings.TrimSpace(string(want)) {
		t.Fatalf("Head = %q, %v, want %q", got, err, want)
	}
}
