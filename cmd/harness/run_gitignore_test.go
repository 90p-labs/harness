package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const gitignoreAdded = "gitignore: added .harness/runs/"

func startRun(t *testing.T) string {
	t.Helper()
	code, stdout, stderr := execute(t, commandRun, "start", "Add the login page")
	if code != 0 {
		t.Fatalf("run start exit %d: %s", code, stderr)
	}
	return stdout
}

func TestRunStartCreatesTheGitignore(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)

	stdout := startRun(t)

	got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != ".harness/runs/\n" {
		t.Fatalf(".gitignore = %q", got)
	}
	if !strings.Contains(stdout, gitignoreAdded+"\n") {
		t.Fatalf("stdout:\n%s\nwant %q", stdout, gitignoreAdded)
	}
}

func TestRunStartAppendsRunsToGitignore(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	writeRepoFile(t, dir, ".gitignore", "node_modules/\n*.log\n")

	stdout := startRun(t)

	got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "node_modules/\n*.log\n.harness/runs/\n" {
		t.Fatalf(".gitignore = %q", got)
	}
	if !strings.Contains(stdout, gitignoreAdded+"\n") {
		t.Fatalf("stdout:\n%s\nwant %q", stdout, gitignoreAdded)
	}
}

func TestRunStartKeepsAnIgnoringGitignore(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	const content = "dist/\n.harness/runs/\n# keep"
	writeRepoFile(t, dir, ".gitignore", content)

	stdout := startRun(t)

	got, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf(".gitignore = %q, want it unchanged", got)
	}
	if strings.Contains(stdout, "gitignore:") {
		t.Fatalf("stdout:\n%s", stdout)
	}
}
