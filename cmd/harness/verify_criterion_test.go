package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyOneCriterionSkipsConfiguredGates(t *testing.T) {
	clearHarnessEnv(t)
	dir, plan := tracedRepo(t)
	cfg := "gates:\n  - id: mark\n    run: \"touch gate-ran.txt; exit 1\"\n"
	if err := os.WriteFile(filepath.Join(dir, configFile), []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	startWithPlan(t, dir, plan)

	code, stdout, stderr := execute(t, commandVerify, "--criterion", "1")

	if code != exitOK || stderr != "" || stdout != "PASS gates=1 cached=0\n" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want only the proof of criterion 1", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "gate-ran.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the configured gate ran: stat error = %v, want the mark file absent", err)
	}
}

func TestVerifyOneCriterionCommand(t *testing.T) {
	clearHarnessEnv(t)
	dir, plan := tracedRepo(t)
	startWithPlan(t, dir, plan)

	code, stdout, stderr := execute(t, commandVerify, "--criterion", "1")
	if code != exitOK || stderr != "" || stdout != "PASS gates=1 cached=0\n" {
		t.Fatalf("first run: code = %d, stdout = %q, stderr = %q, want only criterion-1", code, stdout, stderr)
	}

	code, stdout, _ = execute(t, commandVerify, "--criterion", "1")
	if code != exitOK || stdout != "PASS gates=1 cached=1\n" {
		t.Fatalf("second run: code = %d, stdout = %q, want the evidence kept", code, stdout)
	}

	code, _, stderr = execute(t, commandVerify, "--criterion", "99")
	if code != exitRuntime || !strings.Contains(stderr, "99") {
		t.Fatalf("unknown number: code = %d, stderr = %q, want %d and the number", code, stderr, exitRuntime)
	}

	code, _, _ = execute(t, commandVerify, "--criterion", "1", "--commits", "main")
	if code != exitUsage {
		t.Fatalf("with --commits: code = %d, want %d", code, exitUsage)
	}
}
