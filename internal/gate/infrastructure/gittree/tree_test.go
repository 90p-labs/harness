package gittree_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/gate/infrastructure/gittree"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const (
	goTest       = "go test ./..."
	baselineFile = ".harness-lint/baseline.json"
)

var (
	_ domain.Hasher      = gittree.Tree{}
	_ domain.BatchHasher = gittree.Tree{}
	_ domain.Commits     = gittree.Tree{}
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

// expectedHash computes the input hash of a gate with its own scratch index.
func expectedHash(t *testing.T, dir string, gate domain.Gate) string {
	t.Helper()
	index := filepath.Join(t.TempDir(), "index")
	run := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_INDEX_FILE="+index)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
		return string(out)
	}
	run("add", "-A")
	var files string
	if len(gate.Paths) == 0 {
		files = strings.TrimSpace(run("write-tree"))
	} else {
		patterns := gate.Paths
		if gate.Baseline != "" {
			patterns = append(append([]string{}, gate.Paths...), gate.Baseline)
		}
		var lines []string
		for _, entry := range strings.Split(run("ls-files", "-s", "-z"), "\x00") {
			meta, name, found := strings.Cut(entry, "\t")
			fields := strings.Fields(meta)
			if !found || len(fields) < 2 || !git.MatchesAny(patterns, name) {
				continue
			}
			lines = append(lines, name+" "+fields[0]+" "+fields[1]+"\n")
		}
		sort.Strings(lines)
		files = strings.Join(lines, "")
	}
	sum := sha256.Sum256([]byte(gate.Run + "\x00" + gate.Baseline + "\x00" + files))
	return hex.EncodeToString(sum[:])
}

func TestInputHashes(t *testing.T) {
	ctx := context.Background()
	dir := newRepo(t)
	writeFile(t, dir, "src/a/main.go", "package a\n")
	writeFile(t, dir, "src/b/util.go", "package b\n")
	writeFile(t, dir, "README.md", "one\n")
	writeFile(t, dir, baselineFile, `{"version":1,"findings":[]}`)
	gates := []domain.Gate{
		{Run: "true"},
		{Run: goTest, Paths: []string{"src/**/*.go"}},
		{Run: "lint", Paths: []string{"src/**"}, Baseline: baselineFile},
	}
	want := make([]string, len(gates))
	for i, gate := range gates {
		want[i] = expectedHash(t, dir, gate)
	}

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	logFile := filepath.Join(bin, "calls.log")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + logFile + "'\nexec '" + realGit + "' \"$@\"\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	got, err := gittree.Tree{}.InputHashes(ctx, dir, gates)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("InputHashes = %v, want %v", got, want)
	}

	logged, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatal(err)
	}
	adds := 0
	for _, line := range strings.Split(string(logged), "\n") {
		if first, _, _ := strings.Cut(line, " "); first == "add" {
			adds++
		}
	}
	if adds != 1 {
		t.Fatalf("git add ran %d times, want 1; log:\n%s", adds, logged)
	}

	single, err := gittree.Tree{}.InputHash(ctx, dir, gates[1])
	if err != nil || single != want[1] {
		t.Fatalf("InputHash = %q, %v, want %q", single, err, want[1])
	}

	// A gate without paths: the single and the batch form agree with the expected hash.
	noPaths, err := gittree.Tree{}.InputHash(ctx, dir, gates[0])
	if err != nil || noPaths != want[0] {
		t.Fatalf("InputHash without paths = %q, %v, want %q", noPaths, err, want[0])
	}
	noPathsBatch, err := gittree.Tree{}.InputHashes(ctx, dir, gates[:1])
	if err != nil || len(noPathsBatch) != 1 || noPathsBatch[0] != noPaths {
		t.Fatalf("InputHashes without paths = %v, %v, want [%q]", noPathsBatch, err, noPaths)
	}

	// An empty list must not run git at all: truncate the log, call, read the log.
	for name, empty := range map[string][]domain.Gate{"nil": nil, "empty slice": {}} {
		if err := os.WriteFile(logFile, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		none, err := gittree.Tree{}.InputHashes(ctx, dir, empty)
		if err != nil || len(none) != 0 {
			t.Fatalf("InputHashes(%s) = %v, %v, want empty", name, none, err)
		}
		after, err := os.ReadFile(logFile)
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(string(after)) != "" {
			t.Fatalf("InputHashes(%s) ran git:\n%s", name, after)
		}
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

func commitEmpty(t *testing.T, dir, message string) string {
	t.Helper()
	gitIn(t, dir, "commit", "-q", "--allow-empty", "-m", message)
	return gitIn(t, dir, "rev-parse", "HEAD")
}

func TestResolveBaseReturnsAProbeError(t *testing.T) {
	dir := newRepo(t)
	gitIn(t, dir, "checkout", "-q", "-b", "main")
	commitEmpty(t, dir, "root")
	gitIn(t, dir, "update-ref", "--no-deref", "refs/remotes/origin/main", "HEAD")
	ref := filepath.Join(dir, ".git", "refs", "remotes", "origin", "main")
	writeFile(t, dir, filepath.Join(".git", "refs", "remotes", "origin", "main"), strings.Repeat("1", 40)+"\n")

	got, err := gittree.Tree{}.ResolveBase(context.Background(), dir, "main")

	if err == nil {
		t.Fatalf("ResolveBase over a broken remote ref %s = %+v, nil, want an error", ref, got)
	}
	if !strings.Contains(err.Error(), "origin/main") {
		t.Fatalf("error = %v, want it to name origin/main", err)
	}
}

func TestResolveBaseReturnsAMergeBaseError(t *testing.T) {
	const missing = "no-such-branch"
	dir := newRepo(t)
	gitIn(t, dir, "checkout", "-q", "-b", "main")
	commitEmpty(t, dir, "root")

	got, err := gittree.Tree{}.ResolveBase(context.Background(), dir, missing)

	if err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("ResolveBase(%q) = %+v, %v, want an error naming %s", missing, got, err, missing)
	}
}

func TestResolveBase(t *testing.T) {
	const (
		mainBranch = "main"
		originMain = "origin/main"
		remoteRef  = "refs/remotes/origin/main"
	)
	ctx := context.Background()
	tree := gittree.Tree{}
	newBranchRepo := func(t *testing.T) (dir, root string) {
		dir = newRepo(t)
		gitIn(t, dir, "checkout", "-q", "-b", mainBranch)
		root = commitEmpty(t, dir, "root")
		return dir, root
	}

	cases := map[string]func(t *testing.T) (dir, base string, want domain.Base){
		"a stale local base gives the newer remote base": func(t *testing.T) (string, string, domain.Base) {
			dir, _ := newBranchRepo(t)
			gitIn(t, dir, "checkout", "-q", "-b", "feature")
			upstream := commitEmpty(t, dir, "upstream")
			gitIn(t, dir, "update-ref", remoteRef, upstream)
			commitEmpty(t, dir, "feat: work")
			return dir, mainBranch, domain.Base{Ref: originMain, SHA: upstream}
		},
		"a local base ahead of the remote base keeps the local base": func(t *testing.T) (string, string, domain.Base) {
			dir, root := newBranchRepo(t)
			gitIn(t, dir, "update-ref", remoteRef, root)
			local := commitEmpty(t, dir, "local only")
			gitIn(t, dir, "checkout", "-q", "-b", "feature")
			commitEmpty(t, dir, "feat: work")
			return dir, mainBranch, domain.Base{Ref: mainBranch, SHA: local}
		},
		"no remote base gives the local base": func(t *testing.T) (string, string, domain.Base) {
			dir, root := newBranchRepo(t)
			gitIn(t, dir, "checkout", "-q", "-b", "feature")
			commitEmpty(t, dir, "feat: work")
			return dir, mainBranch, domain.Base{Ref: mainBranch, SHA: root}
		},
		"a sha base keeps the sha": func(t *testing.T) (string, string, domain.Base) {
			dir, root := newBranchRepo(t)
			gitIn(t, dir, "update-ref", remoteRef, root)
			commitEmpty(t, dir, "feat: work")
			return dir, root, domain.Base{Ref: root, SHA: root}
		},
		"a revision keeps its ref": func(t *testing.T) (string, string, domain.Base) {
			dir, _ := newBranchRepo(t)
			gitIn(t, dir, "update-ref", remoteRef, gitIn(t, dir, "rev-parse", "HEAD"))
			previous := commitEmpty(t, dir, "second")
			commitEmpty(t, dir, "feat: work")
			return dir, "HEAD~1", domain.Base{Ref: "HEAD~1", SHA: previous}
		},
		"an origin base keeps its ref": func(t *testing.T) (string, string, domain.Base) {
			dir, root := newBranchRepo(t)
			gitIn(t, dir, "update-ref", remoteRef, root)
			commitEmpty(t, dir, "feat: work")
			return dir, originMain, domain.Base{Ref: originMain, SHA: root}
		},
		"an origin base ignores a remote ref named origin/origin/main": func(t *testing.T) (string, string, domain.Base) {
			dir, root := newBranchRepo(t)
			gitIn(t, dir, "update-ref", remoteRef, root)
			work := commitEmpty(t, dir, "feat: work")
			gitIn(t, dir, "update-ref", "refs/remotes/origin/"+originMain, work)
			return dir, originMain, domain.Base{Ref: originMain, SHA: root}
		},
		"a HEAD base ignores the remote HEAD": func(t *testing.T) (string, string, domain.Base) {
			dir, _ := newBranchRepo(t)
			gitIn(t, dir, "checkout", "-q", "-b", "feature")
			head := commitEmpty(t, dir, "feat: work")
			gitIn(t, dir, "update-ref", "refs/remotes/origin/HEAD", head)
			return dir, "HEAD", domain.Base{Ref: "HEAD", SHA: head}
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			dir, base, want := setup(t)

			got, err := tree.ResolveBase(ctx, dir, base)

			if err != nil || got != want {
				t.Fatalf("ResolveBase(%q) = %+v, %v, want %+v", base, got, err, want)
			}
		})
	}
}
