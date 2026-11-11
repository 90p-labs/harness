package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	gateRunID    = "r1"
	gateTreeOld  = "└── b.go   new"
	gateTreeNew  = "└── b.go   changed"
	gateCriteria = "It works."
	gateBroken   = "It does not."
	gateDirMode  = 0o755
	gateGoalOld  = "Fix the thing."
	gateGoalNew  = "Fix another thing."
	gatePlan     = "# Plan: x\n\n## Goal\n\nTrack: full\n\nFix the thing.\n\n## Acceptance criteria\n\n| # | Criterion | Proof |\n|---|---|---|\n| 1 | It works. | `go test ./...` |\n\n" +
		"## Tasks\n\n### 1. Build\n\n- Owned files: `a.go`, `a_test.go`, `b.go`\n- Test first: `a_test.go` holds `TestA`\n\n" +
		"## Decisions\n\n- 2026-11-10: Use a map.\n\n### Final code tree\n\n```tree\nharness/\n├── a.go   new\n└── b.go   new\n```\n\n" +
		"### Final test tree\n\n```tree\nharness/\n└── a_test.go   new\n```\n"
)

func gateFixture(t *testing.T) (planGate, domain.Run) {
	t.Helper()
	gate := planGate{worktree: t.TempDir()}
	run := domain.Run{ID: gateRunID}
	writeGateFile(t, planPath(gate.worktree, run.ID), gatePlan)
	return gate, run
}

func writeGateFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), gateDirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), keptPlanMode); err != nil {
		t.Fatal(err)
	}
}

func gateHash(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func TestPlanGateKeepCopiesThePlan(t *testing.T) {
	gate, run := gateFixture(t)
	if err := gate.Keep(run); err != nil {
		t.Fatal(err)
	}
	kept, err := os.ReadFile(keptPlanPath(gate.worktree, run.ID))
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != gatePlan {
		t.Fatalf("kept copy differs from plan.md:\n%s", kept)
	}
}

func TestPlanGateStructureKept(t *testing.T) {
	treeEdit := strings.Replace(gatePlan, gateTreeOld, gateTreeNew, 1)
	criterionEdit := strings.Replace(gatePlan, gateCriteria, gateBroken, 1)
	goalEdit := strings.Replace(gatePlan, gateGoalOld, gateGoalNew, 1)
	tests := []struct {
		name    string
		keep    bool
		kept    string
		current string
		hash    string
		want    bool
	}{
		{name: "tree edit after keep", keep: true, current: treeEdit, hash: gateHash(gatePlan), want: true},
		{name: "goal prose edit after keep", keep: true, current: goalEdit, hash: gateHash(gatePlan), want: true},
		{name: "criterion edit after keep", keep: true, current: criterionEdit, hash: gateHash(gatePlan), want: false},
		{name: "kept copy edited after keep", keep: true, kept: treeEdit, current: gatePlan, hash: gateHash(gatePlan), want: false},
		{name: "wrong approved hash", keep: true, current: treeEdit, hash: gateHash(treeEdit), want: false},
		{name: "kept copy missing", current: treeEdit, hash: gateHash(gatePlan), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gate, run := gateFixture(t)
			if tt.keep {
				if err := gate.Keep(run); err != nil {
					t.Fatal(err)
				}
			}
			if tt.kept != "" {
				writeGateFile(t, keptPlanPath(gate.worktree, run.ID), tt.kept)
			}
			writeGateFile(t, planPath(gate.worktree, run.ID), tt.current)
			got, err := gate.StructureKept(run, tt.hash)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("StructureKept = %v, want %v", got, tt.want)
			}
		})
	}
}
