package git_test

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
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
	const github = "github.com/90p-labs/harness"
	cases := map[string]struct{ url, want string }{
		"scp":             {"git@github.com:90p-labs/harness.git", github},
		"https with user": {"https://user@github.com/90p-labs/harness.git", github},
		"https":           {"https://github.com/90p-labs/harness", github},
		"ssh with port":   {"ssh://git@github.com:22/90p-labs/harness", github},
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

	// A repository cloned without origin/HEAD, or never pushed, still has its own default branch.
	for _, branch := range []string{"master", "main"} {
		t.Run("origin HEAD not set, only "+branch, func(t *testing.T) {
			root := initRepo(t)
			for _, args := range [][]string{
				{"symbolic-ref", "HEAD", "refs/heads/" + branch},
				{"commit", "-q", "--allow-empty", "-m", "init"},
				{"checkout", "-q", "-b", "feature"},
			} {
				if out, err := exec.Command("git", append([]string{"-C", root, "-c", "user.name=t", "-c", "user.email=t@t"}, args...)...).CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v: %s", args, err, out)
				}
			}

			got, err := git.DefaultBranch(context.Background(), root)

			if err != nil || got != branch {
				t.Fatalf("DefaultBranch = %q, %v, want %s", got, err, branch)
			}
		})
	}

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

func TestFiles(t *testing.T) {
	root := initRepo(t)
	write := func(name string) {
		t.Helper()
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".gitignore")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"src/a.tsx", "src/deep/er/b.tsx", "src/c.ts", "README.md", "src/gone.tsx"} {
		write(name)
	}
	if out, err := exec.Command("git", "-C", root, "add", "src/a.tsx", "README.md", "src/gone.tsx").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	if err := os.Remove(filepath.Join(root, "src/gone.tsx")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lib/d.tsx", "ignored/e.tsx"} {
		write(name)
	}

	cases := []struct {
		name  string
		globs []string
		want  []string
	}{
		{
			name: "no globs lists every file but the ignored ones",
			want: []string{".gitignore", "README.md", "lib/d.tsx", "src/a.tsx", "src/c.ts", "src/deep/er/b.tsx"},
		},
		{
			name:  "double star crosses folders, tracked and untracked",
			globs: []string{"src/**/*.tsx"},
			want:  []string{"src/a.tsx", "src/deep/er/b.tsx"},
		},
		{
			name:  "several globs",
			globs: []string{"lib/*.tsx", "README.md"},
			want:  []string{"README.md", "lib/d.tsx"},
		},
		{name: "no match", globs: []string{"docs/**"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := git.Files(context.Background(), root, tc.globs...)

			if err != nil {
				t.Fatalf("Files: %v", err)
			}
			sort.Strings(got)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("Files = %v, want %v", got, tc.want)
			}
		})
	}

	outside, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := git.Files(context.Background(), outside); !errors.Is(err, git.ErrNotRepository) {
		t.Fatalf("outside a repo: %v, want ErrNotRepository", err)
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

func gitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	full := append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)
	if out, err := exec.Command("git", full...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}

func commitFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, dir, "add", name)
	gitCommand(t, dir, "commit", "-q", "-m", name)
}

func TestBranchRef(t *testing.T) {
	ctx := context.Background()
	dir := initRepo(t)
	commitFile(t, dir, "a.txt")
	commitFile(t, dir, "b.txt")
	gitCommand(t, dir, "branch", "feature")
	gitCommand(t, dir, "update-ref", "refs/remotes/origin/remote-only", "HEAD")
	gitCommand(t, dir, "tag", "tag-only", "HEAD~1")
	gitCommand(t, dir, "branch", "both", "HEAD")
	gitCommand(t, dir, "tag", "both", "HEAD~1")

	ok := map[string]string{
		"feature":     "refs/heads/feature",
		"remote-only": "refs/remotes/origin/remote-only",
		"both":        "refs/heads/both",
	}
	for name, want := range ok {
		got, err := git.BranchRef(ctx, dir, name)
		if err != nil || got != want {
			t.Errorf("BranchRef(%q) = %q, %v, want %q", name, got, err, want)
		}
	}
	for _, name := range []string{"HEAD~1", "tag-only", "-x"} {
		if got, err := git.BranchRef(ctx, dir, name); !errors.Is(err, git.ErrNotBranch) {
			t.Errorf("BranchRef(%q) = %q, %v, want ErrNotBranch", name, got, err)
		}
	}
}

func TestBranchRefNotARepoIsNotMissing(t *testing.T) {
	got, err := git.BranchRef(context.Background(), t.TempDir(), "main")
	if err == nil || errors.Is(err, git.ErrNotBranch) {
		t.Errorf("BranchRef in a non-repo dir = %q, %v, want a git error that is not ErrNotBranch", got, err)
	}
}

func TestPatchIDAndChangedFiles(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	gitCommand(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, root, "base.txt")

	empty, err := git.PatchID(ctx, root, "main")
	if err != nil || empty != "" {
		t.Fatalf("PatchID on the base = %q, %v, want empty", empty, err)
	}
	if files, err := git.ChangedFiles(ctx, root, "main"); err != nil || len(files) != 0 {
		t.Fatalf("ChangedFiles on the base = %v, %v, want none", files, err)
	}

	gitCommand(t, root, "checkout", "-q", "-b", "feature")
	commitFile(t, root, "one.txt")
	commitFile(t, root, "two.txt")
	before, err := git.PatchID(ctx, root, "main")
	if err != nil || before == "" {
		t.Fatalf("PatchID = %q, %v, want an id", before, err)
	}
	files, err := git.ChangedFiles(ctx, root, "main")
	if err != nil || !slices.Equal(files, []string{"one.txt", "two.txt"}) {
		t.Fatalf("ChangedFiles = %v, %v, want one.txt and two.txt", files, err)
	}

	gitCommand(t, root, "checkout", "-q", "main")
	commitFile(t, root, "unrelated.txt")
	gitCommand(t, root, "checkout", "-q", "feature")
	gitCommand(t, root, "rebase", "-q", "main")
	after, err := git.PatchID(ctx, root, "main")
	if err != nil || after != before {
		t.Fatalf("PatchID after a clean rebase = %q, %v, want %q", after, err, before)
	}

	commitFile(t, root, "three.txt")
	changed, err := git.PatchID(ctx, root, "main")
	if err != nil || changed == "" || changed == before {
		t.Fatalf("PatchID after a new commit = %q, %v, want a new id", changed, err)
	}
}

func TestChangedFilesNoRenamesIgnoresReplaceRefs(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	gitCommand(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, root, "base.txt")

	gitCommand(t, root, "checkout", "-q", "-b", "fake")
	makeDir(t, root, "cmd")
	commitFile(t, root, "cmd/m.go")
	fake := revParse(t, root, "HEAD")

	gitCommand(t, root, "checkout", "-q", "main")
	gitCommand(t, root, "checkout", "-q", "-b", "feature")
	makeDir(t, root, "internal/guard")
	commitFile(t, root, "internal/guard/x.go")
	realTip := revParse(t, root, "HEAD")

	gitCommand(t, root, "replace", realTip, fake)
	if got := revParse(t, root, "HEAD"); got != realTip {
		t.Fatalf("HEAD = %s, want %s", got, realTip)
	}

	files, err := git.ChangedFilesNoRenames(ctx, root, "main")
	if err != nil || !slices.Equal(files, []string{"internal/guard/x.go"}) {
		t.Fatalf("ChangedFilesNoRenames with a replace ref = %v, %v, want internal/guard/x.go", files, err)
	}
}

func TestChangedFilesNoRenamesErrorNamesDiff(t *testing.T) {
	root := initRepo(t)
	commitFile(t, root, "base.txt")

	_, err := git.ChangedFilesNoRenames(context.Background(), root, "no-such-branch")
	if err == nil || !strings.Contains(err.Error(), "git diff in ") {
		t.Fatalf("ChangedFilesNoRenames on a missing base = %v, want an error naming git diff", err)
	}
}

func TestChangedFilesNoRenamesIgnoresGrafts(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	gitCommand(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, root, "base.txt")
	tip := revParse(t, root, "HEAD")

	gitCommand(t, root, "checkout", "-q", "-b", "feature")
	makeDir(t, root, "internal/guard")
	commitFile(t, root, "internal/guard/x.go")
	first := revParse(t, root, "HEAD")
	makeDir(t, root, "cmd")
	commitFile(t, root, "cmd/m.go")

	graftPath := gitPath(t, root, "info/grafts")
	if err := os.WriteFile(graftPath, []byte(tip+" "+first+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	files, err := git.ChangedFilesNoRenames(ctx, root, "refs/heads/main")
	if err != nil || !slices.Contains(files, "internal/guard/x.go") {
		t.Fatalf("ChangedFilesNoRenames with a graft = %v, %v, want internal/guard/x.go", files, err)
	}
}

func TestChangedFilesNoRenamesIgnoresCommitGraph(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	gitCommand(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, root, "b")

	gitCommand(t, root, "checkout", "-q", "-b", "feature")
	makeDir(t, root, "internal/guard")
	commitFile(t, root, "internal/guard/x.go")
	guard := revParse(t, root, "HEAD")
	makeDir(t, root, "cmd")
	commitFile(t, root, "cmd/m.go")

	gitCommand(t, root, "checkout", "-q", "main")
	commitFile(t, root, "m1")
	m1 := revParse(t, root, "HEAD")
	commitFile(t, root, "m2")
	gitCommand(t, root, "checkout", "-q", "feature")

	gitCommand(t, root, "commit-graph", "write", "--reachable", "--no-progress")
	graphPath := gitPath(t, root, "objects/info/commit-graph")
	data, err := os.ReadFile(graphPath)
	if err != nil {
		t.Fatal(err)
	}

	chunks := map[string]uint64{}
	for i := 0; i <= int(data[6]); i++ {
		off := 8 + i*12
		chunks[string(data[off:off+4])] = binary.BigEndian.Uint64(data[off+4 : off+12])
	}
	oidf, oidl, cdat := chunks["OIDF"], chunks["OIDL"], chunks["CDAT"]
	if oidf == 0 || oidl == 0 || cdat == 0 {
		t.Fatal("commit-graph has no OIDF, OIDL or CDAT chunk, so the forge does not apply")
	}
	n := int(binary.BigEndian.Uint32(data[oidf+255*4:]))
	position := func(oid string) uint32 {
		raw, err := hex.DecodeString(oid)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < n; i++ {
			start := int(oidl) + i*20
			if bytes.Equal(data[start:start+20], raw) {
				return uint32(i)
			}
		}
		t.Fatalf("commit %s is not in the commit-graph", oid)
		return 0
	}
	binary.BigEndian.PutUint32(data[int(cdat)+int(position(m1))*36+20:], position(guard))
	sum := sha1.Sum(data[:len(data)-20])
	copy(data[len(data)-20:], sum[:])
	if err := os.Remove(graphPath); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(graphPath, data, 0o600); err != nil {
		t.Fatal(err)
	}

	baseOut, err := exec.Command("git", "-C", root, "merge-base", "main", "HEAD").Output()
	if got := strings.TrimSpace(string(baseOut)); err != nil || got != guard {
		t.Fatalf("merge-base = %q, %v, want %s: the forge depends on the commit-graph format", got, err, guard)
	}

	files, err := git.ChangedFilesNoRenames(ctx, root, "refs/heads/main")
	if err != nil || !slices.Contains(files, "internal/guard/x.go") {
		t.Fatalf("ChangedFilesNoRenames with a forged commit-graph = %v, %v, want internal/guard/x.go", files, err)
	}
}

func TestChangedFilesNoRenamesIgnoresDiffRelative(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	gitCommand(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, root, "b")
	gitCommand(t, root, "config", "diff.relative", "true")

	gitCommand(t, root, "checkout", "-q", "-b", "feature")
	makeDir(t, root, "internal/guard")
	makeDir(t, root, "docs")
	commitFile(t, root, "internal/guard/x.go")
	commitFile(t, root, "docs/a.md")

	files, err := git.ChangedFilesNoRenames(ctx, filepath.Join(root, "docs"), "refs/heads/main")
	if err != nil || !slices.Contains(files, "internal/guard/x.go") {
		t.Fatalf("ChangedFilesNoRenames with diff.relative = %v, %v, want internal/guard/x.go", files, err)
	}
}

func TestChangedFilesNoRenamesIgnoresInheritedGitLocation(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	gitCommand(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, root, "b")
	gitCommand(t, root, "checkout", "-q", "-b", "feature")
	makeDir(t, root, "internal/guard")
	commitFile(t, root, "internal/guard/x.go")

	decoy := initRepo(t)
	gitCommand(t, decoy, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, decoy, "b")
	gitCommand(t, decoy, "checkout", "-q", "-b", "feature")
	makeDir(t, decoy, "docs")
	commitFile(t, decoy, "docs/a.md")

	t.Setenv("GIT_DIR", filepath.Join(decoy, ".git"))
	files, err := git.ChangedFilesNoRenames(ctx, root, "refs/heads/main")
	if err != nil || !slices.Contains(files, "internal/guard/x.go") {
		t.Fatalf("ChangedFilesNoRenames with an inherited GIT_DIR = %v, %v, want internal/guard/x.go", files, err)
	}
}

func TestChangedFilesNoRenamesIgnoresSubmoduleConfig(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	gitCommand(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, root, "b")

	gitCommand(t, root, "checkout", "-q", "-b", "feature")
	gitCommand(t, root, "update-index", "--add", "--cacheinfo", "160000,"+revParse(t, root, "HEAD")+",internal/hooks/sub")
	gitCommand(t, root, "commit", "-q", "-m", "gitlink")
	gitCommand(t, root, "config", "diff.ignoreSubmodules", "all")

	files, err := git.ChangedFilesNoRenames(ctx, root, "refs/heads/main")
	if err != nil || !slices.Contains(files, "internal/hooks/sub") {
		t.Fatalf("ChangedFilesNoRenames with diff.ignoreSubmodules = %v, %v, want internal/hooks/sub", files, err)
	}
}

func makeDir(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, rel), 0o700); err != nil {
		t.Fatal(err)
	}
}

// gitPath resolves a path inside the git dir to an absolute path and makes its parent folder.
func gitPath(t *testing.T, root, rel string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", root, "rev-parse", "--git-path", rel).Output()
	if err != nil {
		t.Fatal(err)
	}
	p := strings.TrimSpace(string(out))
	if !filepath.IsAbs(p) {
		p = filepath.Join(root, p)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	return p
}

func revParse(t *testing.T, dir, rev string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", rev).Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", rev, err)
	}
	return strings.TrimSpace(string(out))
}

func TestPatchIDSeesWhitespace(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	gitCommand(t, root, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFile(t, root, "base.txt")
	gitCommand(t, root, "checkout", "-q", "-b", "feature")
	writeFile := func(content string) {
		if err := os.WriteFile(filepath.Join(root, "x.py"), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("if x:\n  y()\n")
	gitCommand(t, root, "add", "x.py")
	gitCommand(t, root, "commit", "-q", "-m", "x")
	before, err := git.PatchID(ctx, root, "main")
	if err != nil || before == "" {
		t.Fatalf("PatchID = %q, %v, want an id", before, err)
	}

	writeFile("if x:\n    y()\n")
	gitCommand(t, root, "commit", "-q", "-a", "--amend", "--no-edit")
	after, err := git.PatchID(ctx, root, "main")
	if err != nil || after == "" || after == before {
		t.Fatalf("PatchID after a re-indent = %q, %v, want an id other than %q", after, err, before)
	}
}

func TestDiffLines(t *testing.T) {
	ctx := context.Background()
	root := initRepo(t)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("1\n2\n3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "a.txt")
	gitCommand(t, root, "commit", "-q", "-m", "a")
	from, err := git.Head(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := git.DiffLines(ctx, root, from); err != nil || got != 0 {
		t.Fatalf("DiffLines on the same commit = %d, %v, want 0", got, err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("1\nX\n3\n4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "commit", "-q", "-am", "b")
	if got, err := git.DiffLines(ctx, root, from); err != nil || got != 3 {
		t.Fatalf("DiffLines = %d, %v, want 3 (two added, one deleted)", got, err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0, 1, 2, 0}, 0o600); err != nil {
		t.Fatal(err)
	}
	gitCommand(t, root, "add", "bin.dat")
	gitCommand(t, root, "commit", "-q", "-m", "bin")
	if _, err := git.DiffLines(ctx, root, from); err == nil {
		t.Fatal("DiffLines with a binary file: want an error")
	}
	if _, err := git.DiffLines(ctx, root, "nope"); err == nil {
		t.Fatal("DiffLines from an unknown commit: want an error")
	}
}
