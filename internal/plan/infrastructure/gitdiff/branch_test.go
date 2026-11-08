package gitdiff_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/plan/infrastructure/gitdiff"
)

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadBranch(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitIn(t, dir, "init", "-q")
	write(t, dir, "drop.txt", "old\n")
	write(t, dir, "keep.txt", "one\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "base")
	const base = "trunk"
	gitIn(t, dir, "branch", base)

	write(t, dir, "keep.txt", "two\n")
	write(t, dir, "internal/ping/pong.go", "package ping\n")
	gitIn(t, dir, "rm", "-q", "drop.txt")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "feat: add the pong domain")
	first := gitIn(t, dir, "rev-parse", "HEAD")

	write(t, dir, "internal/ping/pong_test.go", "package ping\n\nfunc TestPong(t *testing.T) {}\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "test: cover the pong")
	second := gitIn(t, dir, "rev-parse", "HEAD")

	gitIn(t, dir, "checkout", "-q", base)
	write(t, dir, "later.txt", "on the base after the branch point\n")
	gitIn(t, dir, "add", "-A")
	gitIn(t, dir, "commit", "-q", "-m", "chore: move the base")
	gitIn(t, dir, "checkout", "-q", "-")

	got, err := gitdiff.Reader{}.Read(context.Background(), dir, base,
		[]string{"internal/ping/pong_test.go", "internal/ping/absent_test.go"})
	if err != nil {
		t.Fatal(err)
	}

	want := domain.Branch{
		Files: []domain.FileChange{
			{Path: "drop.txt", Status: domain.StatusDeleted},
			{Path: "internal/ping/pong.go", Status: domain.StatusAdded},
			{Path: "internal/ping/pong_test.go", Status: domain.StatusAdded},
			{Path: "keep.txt", Status: domain.StatusModified},
		},
		Commits: []domain.BranchCommit{
			{SHA: first, Subject: "feat: add the pong domain", Files: []string{"drop.txt", "internal/ping/pong.go", "keep.txt"}},
			{SHA: second, Subject: "test: cover the pong", Files: []string{"internal/ping/pong_test.go"}},
		},
		Tests: map[string]string{"internal/ping/pong_test.go": "package ping\n\nfunc TestPong(t *testing.T) {}\n"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("branch = %+v\nwant     %+v", got, want)
	}
}
