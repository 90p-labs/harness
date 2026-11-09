package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	programFixture = "../../internal/plan/domain/testdata/program.md"
	programRunIDs  = "R1 R2 R3a R4 R5 R6 R3b R7"
	flagFile       = "--file"
	flagOut        = "--out"
)

func TestPlanProgram(t *testing.T) {
	clearHarnessEnv(t)
	clearOTLPEnv(t)
	raw, err := os.ReadFile(programFixture)
	if err != nil {
		t.Fatal(err)
	}
	dir := initRepo(t)
	plan := filepath.Join(dir, "program.md")
	if err := os.WriteFile(plan, raw, 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(t, commandPlan, commandCheck, flagFile, plan)
	if code != exitOK || stdout != "" || stderr != "" {
		t.Fatalf("check: code = %d, stdout = %q, stderr = %q, want a clean program", code, stdout, stderr)
	}

	out := filepath.Join(t.TempDir(), "page.html")
	code, stdout, stderr = execute(t, commandPlan, "view", flagFile, plan, flagOut, out)
	if code != exitOK || strings.TrimSpace(stdout) != out || stderr != "" {
		t.Fatalf("view: code = %d, stdout = %q, stderr = %q, want %q", code, stdout, stderr, out)
	}
	page, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range strings.Fields(programRunIDs) {
		if !strings.Contains(string(page), `id="run-`+id+`"`) {
			t.Errorf("the page has no card for %s", id)
		}
	}
}
