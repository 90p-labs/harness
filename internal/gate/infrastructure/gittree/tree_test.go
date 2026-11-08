package gittree_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/gate/infrastructure/gittree"
)

const (
	goTest       = "go test ./..."
	baselineFile = ".harness-baseline/lint.json"
)

var (
	_ domain.Hasher  = gittree.Tree{}
	_ domain.Commits = gittree.Tree{}
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newRepo(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "init", "-q")
	return dir
}

func TestInputHash(t *testing.T) {
	ctx := context.Background()
	tree := gittree.Tree{}
	dir := newRepo(t)
	writeFile(t, dir, "src/a/main.go", "package a\n")
	writeFile(t, dir, "src/b/util.go", "package b\n")
	writeFile(t, dir, "README.md", "one\n")
	paths := []string{"src/**/*.go"}

	base, err := tree.InputHash(ctx, dir, domain.Gate{Run: goTest, Paths: paths})
	if err != nil {
		t.Fatal(err)
	}
	again, err := tree.InputHash(ctx, dir, domain.Gate{Run: goTest, Paths: paths})
	if err != nil || again != base {
		t.Fatalf("second hash = %q, %v, want %q", again, err, base)
	}

	writeFile(t, dir, "README.md", "two\n")
	if got, _ := tree.InputHash(ctx, dir, domain.Gate{Run: goTest, Paths: paths}); got != base {
		t.Fatalf("hash changed after a non-matching file changed")
	}

	writeFile(t, dir, "src/a/main.go", "package a // edited\n")
	changed, err := tree.InputHash(ctx, dir, domain.Gate{Run: goTest, Paths: paths})
	if err != nil || changed == base {
		t.Fatalf("hash = %q, %v, want a value different from %q", changed, err, base)
	}

	if staged := gitIn(t, dir, "diff", "--cached", "--name-only"); staged != "" {
		t.Fatalf("real index has staged files: %q", staged)
	}
	if _, err := os.Stat(filepath.Join(dir, ".git", "index")); !os.IsNotExist(err) {
		t.Fatalf("real index file was created: %v", err)
	}
}

func TestInputHashGlobs(t *testing.T) {
	ctx := context.Background()
	tree := gittree.Tree{}
	dir := newRepo(t)
	writeFile(t, dir, "main.go", "package main\n")
	writeFile(t, dir, "pkg/deep/x.go", "package deep\n")
	writeFile(t, dir, "pkg/deep/x.txt", "text\n")

	hash := func(paths ...string) string {
		t.Helper()
		got, err := tree.InputHash(ctx, dir, domain.Gate{Run: goTest, Paths: paths})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	topOnly := hash("*.go")
	writeFile(t, dir, "pkg/deep/x.go", "package deep // edited\n")
	if hash("*.go") != topOnly {
		t.Fatal("single star glob matched a nested file")
	}
	recursive := hash("**/*.go")
	writeFile(t, dir, "pkg/deep/x.go", "package deep // edited again\n")
	if hash("**/*.go") == recursive {
		t.Fatal("double star glob missed a nested file")
	}
	writeFile(t, dir, "pkg/deep/x.txt", "other\n")
	before := hash("pkg/**")
	writeFile(t, dir, "pkg/deep/x.txt", "other again\n")
	if hash("pkg/**") == before {
		t.Fatal("trailing double star missed a nested file")
	}
}

func TestInputHashWithoutPathsCoversTheTree(t *testing.T) {
	ctx := context.Background()
	dir := newRepo(t)
	writeFile(t, dir, "a.txt", "a\n")
	whole := domain.Gate{Run: goTest}

	before, err := gittree.Tree{}.InputHash(ctx, dir, whole)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "deep/testdata/b.txt", "b\n")
	if after, err := (gittree.Tree{}).InputHash(ctx, dir, whole); err != nil || after == before {
		t.Fatalf("InputHash = %q, %v, want a change after a new file", after, err)
	}
}

func TestInputHashCoversTheGateInputs(t *testing.T) {
	ctx := context.Background()
	tree := gittree.Tree{}
	dir := newRepo(t)
	writeFile(t, dir, "src/a.go", "package a\n")
	writeFile(t, dir, baselineFile, `{"version":1,"findings":[]}`)
	gate := domain.Gate{Run: goTest, Paths: []string{"src/**"}, Baseline: baselineFile}
	hash := func(g domain.Gate) string {
		t.Helper()
		got, err := tree.InputHash(ctx, dir, g)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	base := hash(gate)

	other := gate
	other.Run = "true"
	if hash(other) == base {
		t.Error("the hash ignores the run command")
	}
	unbaselined := gate
	unbaselined.Baseline = ""
	if hash(unbaselined) == base {
		t.Error("the hash ignores the baseline path")
	}
	writeFile(t, dir, baselineFile, `{"version":1,"findings":[{"rule":"r"}]}`)
	edited := hash(gate)
	if edited == base {
		t.Error("the hash ignores the baseline content outside the paths")
	}
	if err := os.Chmod(filepath.Join(dir, "src/a.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if hash(gate) == edited {
		t.Error("the hash ignores the file mode")
	}
}

func TestCommits(t *testing.T) {
	ctx := context.Background()
	tree := gittree.Tree{}
	dir := newRepo(t)
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "base")
	base := gitIn(t, dir, "rev-parse", "HEAD")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "second commit")
	second := gitIn(t, dir, "rev-parse", "HEAD")
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", "third commit")
	third := gitIn(t, dir, "rev-parse", "HEAD")

	t.Run("head", func(t *testing.T) {
		got, err := tree.Head(ctx, dir)
		if err != nil || got != third {
			t.Fatalf("Head = %q, %v, want %q", got, err, third)
		}
	})

	t.Run("list oldest first", func(t *testing.T) {
		got, err := tree.List(ctx, dir, base)
		want := []domain.Commit{{SHA: second, Subject: "second commit"}, {SHA: third, Subject: "third commit"}}
		if err != nil || len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("List = %v, %v, want %v", got, err, want)
		}
	})

	t.Run("list empty range", func(t *testing.T) {
		got, err := tree.List(ctx, dir, third)
		if err != nil || len(got) != 0 {
			t.Fatalf("List = %v, %v, want none", got, err)
		}
	})

	t.Run("worktree add and remove", func(t *testing.T) {
		path, err := tree.AddWorktree(ctx, dir, second)
		if err != nil {
			t.Fatal(err)
		}
		if head := gitIn(t, path, "rev-parse", "HEAD"); head != second {
			t.Fatalf("worktree HEAD = %q, want %q", head, second)
		}
		if err := tree.RemoveWorktree(ctx, dir, path); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("worktree still exists: %v", err)
		}
	})
}
