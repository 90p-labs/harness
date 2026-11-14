package main

import (
	"strings"
	"testing"
)

const (
	redSymptom   = "want pong, got ping"
	redProofOld  = "`go test ./internal/ping/domain -run TestPong -count=1`"
	redProofNew  = "`go test ./... -run TestPong -count=1`"
	redGoMod     = "go.mod"
	redTestFile  = "pong_test.go"
	redHead      = "package pong\n\nimport \"testing\"\n\n"
	redBugEdit   = "Track: full\n"
	redTasksHead = "## Tasks\n"
	redSection   = "## Reproduction\n\n" +
		"- Criterion: 1\n" +
		"- Symptom: `" + redSymptom + "`\n" +
		"- Regression test: `TestPong`\n" +
		"\n1. If the cache is stale, then a rerun passes. Result: refuted.\n" +
		"2. If the domain returns the wrong word, then the test shows ping. Result: confirmed.\n" +
		"3. If the query drops the word, then the span is empty. Result: untested.\n\n"
)

func redPlan(t *testing.T) string {
	t.Helper()
	replace := func(plan, old, repl string) string {
		t.Helper()
		if !strings.Contains(plan, old) {
			t.Fatalf("redPlan: the pattern %q is not in the plan", old)
		}
		return strings.Replace(plan, old, repl, 1)
	}
	plan := validPlan(t)
	plan = replace(plan, redBugEdit, redBugEdit+"Cause: bug\n")
	plan = replace(plan, redTasksHead, redSection+redTasksHead)
	plan = replace(plan, redProofOld, redProofNew)
	for _, proof := range []string{
		"`go test ./internal/ping/application/query/pingquery -run TestPingQuery -count=1`",
		"`make check`",
	} {
		plan = replace(plan, proof, trueProof)
	}
	return plan
}

func TestReproductionRedRecordGoesStale(t *testing.T) {
	plan := redPlan(t)
	clearHarnessEnv(t)
	dir := initRepo(t)
	gitIn(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFiles(t, dir, "chore: start the repo", map[string]string{redGoMod: "module pong\n\ngo 1.22\n"})
	_, stdout, _ := execute(t, commandRun, "start", "Fix the pong word")
	runID := startedRunID(t, stdout)
	writeRunPlan(t, dir, runID, plan)
	writeSelfCheck(t, dir, runID)
	red := redHead + "func TestPong(t *testing.T) { t.Fatal(\"" + redSymptom + "\") }\n"
	commitFiles(t, dir, "test: red for the symptom", map[string]string{redTestFile: red})
	if code, _, _ := execute(t, commandVerify, "--criterion", "1"); code == 0 {
		t.Fatal("verify passed, want a red proof")
	}

	commitFiles(t, dir, "test: change a file the proof reads", map[string]string{redTestFile: red + "\n// changed\n"})

	code, _, stderr := execute(t, commandRun, commandPlan)
	if code != exitRuntime || !strings.Contains(stderr, "reproduction-red") || !strings.Contains(stderr, "no record for the current files") {
		t.Fatalf("stale record: code = %d, stderr = %q, want %d and the no-record message", code, stderr, exitRuntime)
	}
}

func TestReproductionRedRunPlan(t *testing.T) {
	clearHarnessEnv(t)
	plan := redPlan(t)
	dir := initRepo(t)
	gitIn(t, dir, "symbolic-ref", "HEAD", "refs/heads/main")
	commitFiles(t, dir, "chore: start the repo", map[string]string{redGoMod: "module pong\n\ngo 1.22\n"})
	_, stdout, _ := execute(t, commandRun, "start", "Fix the pong word")
	runID := startedRunID(t, stdout)
	writeRunPlan(t, dir, runID, plan)
	writeSelfCheck(t, dir, runID)

	tests := []struct {
		name        string
		test        string
		want        string
		proofPasses bool // the proof exits 0, so the plan is refused because it does not reproduce the bug
	}{
		{"no record", "", "harness verify --criterion 1", false},
		{"a missing test exits 0", redHead + "func TestOther(t *testing.T) {}\n", "does not reproduce the bug", true},
		{"a build error", redHead + "func TestPong(t *testing.T) { undefinedWord() }\n", "does not hold", false},
		{"a failure with another text", redHead + "func TestPong(t *testing.T) { t.Fatal(\"the cache is cold\") }\n", "does not hold", false},
	}
	for _, tc := range tests {
		if tc.test != "" {
			commitFiles(t, dir, "test: "+tc.name, map[string]string{redTestFile: tc.test})
			if code, _, _ := execute(t, commandVerify, "--criterion", "1"); (code == 0) != tc.proofPasses {
				t.Fatalf("%s: verify exit code %d, want the proof to pass = %v", tc.name, code, tc.proofPasses)
			}
		}
		code, _, stderr := execute(t, commandRun, commandPlan)
		if code != exitRuntime || !strings.Contains(stderr, "reproduction-red") || !strings.Contains(stderr, tc.want) {
			t.Fatalf("%s: code = %d, stderr = %q, want %d and %q", tc.name, code, stderr, exitRuntime, tc.want)
		}
	}

	commitFiles(t, dir, "test: red for the symptom", map[string]string{redTestFile: redHead + "func TestPong(t *testing.T) { t.Fatal(\"" + redSymptom + "\") }\n"})
	execute(t, commandVerify, "--criterion", "1")
	if code, _, stderr := execute(t, commandRun, commandPlan); code != exitOK {
		t.Fatalf("red proof: code = %d, stderr = %q, want the plan accepted", code, stderr)
	}
}
