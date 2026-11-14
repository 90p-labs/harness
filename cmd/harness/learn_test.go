package main

import (
	"path/filepath"
	"strings"
	"testing"
)

const (
	noSeamRegression = "- Regression test: `TestPong`"
	noSeamReason     = "the pong cache has no seam"
	noSeamLine       = "candidate: no-seam reproduction plan.md: " + noSeamReason
)

func startLearnRun(t *testing.T, plan string) {
	t.Helper()
	clearHarnessEnv(t)
	dir := initRepo(t)
	gitIn(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFiles(t, dir, "chore: start the repo", map[string]string{redGoMod: "module pong\n\ngo 1.22\n"})
	_, stdout, _ := execute(t, commandRun, "start", "Fix the pong word")
	if plan != "" {
		writeRunPlan(t, dir, startedRunID(t, stdout), plan)
	}
}

func learnOutput(t *testing.T) string {
	t.Helper()
	code, stdout, stderr := execute(t, commandRun, "learn")
	if code != exitOK {
		t.Fatalf("learn: code = %d, stderr = %q", code, stderr)
	}
	return stdout
}

func TestNoSeamLearnCommandOtherWorktree(t *testing.T) {
	bug := strings.Replace(redPlan(t), noSeamRegression, "- Regression test: none: "+noSeamReason, 1)
	clearHarnessEnv(t)
	dir := initRepo(t)
	gitIn(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFiles(t, dir, "chore: start the repo", map[string]string{redGoMod: "module pong\n\ngo 1.22\n"})
	_, stdout, _ := execute(t, commandRun, "start", "Fix the pong word")
	runID := startedRunID(t, stdout)
	writeRunPlan(t, dir, runID, bug)
	other := filepath.Join(t.TempDir(), "other")
	gitIn(t, dir, "worktree", "add", "--quiet", "-b", "other", other)
	t.Chdir(other)

	code, out, stderr := execute(t, commandRun, "learn", runID)

	if code != exitOK || !strings.Contains(out, noSeamLine+"\n") {
		t.Fatalf("learn: code = %d, stdout = %q, stderr = %q, want the no-seam line", code, out, stderr)
	}
}

func TestNoSeamLearnCommand(t *testing.T) {
	bug := strings.Replace(redPlan(t), noSeamRegression, "- Regression test: none: "+noSeamReason, 1)
	notBug := strings.Replace(bug, "Cause: bug\n", "", 1)

	for _, tc := range []struct {
		name string
		plan string
		want bool
	}{
		{"a bug plan with no seam", bug, true},
		{"a bug plan with a named test", redPlan(t), false},
		{"a plan with no Cause line", notBug, false},
		{"a plan with no section", validPlan(t), false},
		{"a run folder with no plan", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			startLearnRun(t, tc.plan)

			out := learnOutput(t)

			if got := strings.Contains(out, noSeamLine+"\n"); got != tc.want {
				t.Fatalf("stdout = %q, want the no-seam line: %v", out, tc.want)
			}
			if !tc.want && strings.Contains(out, "no-seam") {
				t.Fatalf("stdout = %q, want no no-seam candidate", out)
			}
		})
	}
}
