package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestVerifyCommitsAgainstAStaleBase(t *testing.T) {
	clearHarnessEnv(t)
	upstream, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	gitIn(t, upstream, "init", "--quiet")
	gitIn(t, upstream, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFiles(t, upstream, "chore: start the repo", nil)

	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "clone")
	gitIn(t, parent, "clone", "--quiet", upstream, dir)
	t.Chdir(dir)
	gitIn(t, dir, "checkout", "--quiet", "-b", "feature")
	commitFiles(t, dir, "feat: add the widget", map[string]string{"broken.txt": "yes\n"})

	commitFiles(t, upstream, "chore: another pull request lands", nil)
	gitIn(t, dir, "fetch", "--quiet", "origin")
	gitIn(t, dir, "-c", gitUserName, "-c", gitUserEmail, "rebase", "--quiet", "origin/main")

	cfg := "git:\n  default_branch: main\ngates:\n  - id: test\n    run: \"if [ -f broken.txt ]; then echo 'FAIL: TestWidget expected 2 got 3'; exit 1; fi\"\n"
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(t, commandVerify, "--commits", "main")

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if code != exitRuntime || !strings.Contains(stderr, "must pass: failed test") {
		t.Fatalf("code = %d, stderr = %q, want the commit rule to break on the test gate", code, stderr)
	}
	if !regexp.MustCompile(`^base origin/main [0-9a-f]{12}$`).MatchString(lines[0]) {
		t.Fatalf("first line = %q, want the origin/main base line", lines[0])
	}
	if strings.Contains(stdout, "another pull request") || strings.Count(stdout, "feat") != 1 || strings.Contains(stdout, "chore") {
		t.Fatalf("stdout = %q, want only the branch commit", stdout)
	}
	if !regexp.MustCompile(`(?m)^FAIL [0-9a-f]{12} feat\n {2}FAIL test \S+\n`).MatchString(stdout) {
		t.Fatalf("stdout = %q, want the feat commit with its failed gate", stdout)
	}
	if !regexp.MustCompile(`(?m)^[0-9a-f]{12} test:\nFAIL: TestWidget expected 2 got 3\n`).MatchString(stdout) {
		t.Fatalf("stdout = %q, want the tail of the failed gate", stdout)
	}
}
