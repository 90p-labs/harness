package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	lintdomain "github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	commandLint      = "lint"
	commandBaseline  = "baseline"
	commandCatalogue = "catalogue"
	flagLintJSON     = "--json"
	todoLintScript   = "testdata/todo-lint.sh"
	todoRuleID       = "team/no-todo"
	todoSourceFile   = "src/orders/order.ts"
	lintBaselineFile = ".harness-baseline/lint.json"
)

func TestLintCatalogue(t *testing.T) {
	clearHarnessEnv(t)
	t.Chdir(t.TempDir())

	code, stdout, stderr := execute(t, commandLint, commandCatalogue, flagLintJSON)

	var catalogue lintdomain.RuleCatalogue
	if code != exitOK || json.Unmarshal([]byte(stdout), &catalogue) != nil || len(catalogue.Packs) == 0 {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want the catalogue outside a repository", code, stdout, stderr)
	}
}

func TestLintExecPlugin(t *testing.T) {
	clearHarnessEnv(t)
	script, err := filepath.Abs(todoLintScript)
	if err != nil {
		t.Fatal(err)
	}
	dir := initRepo(t)
	commitFiles(t, dir, "chore: lint plugin repo", map[string]string{
		".harness.yaml": `plugins:
  - name: todo-lint
    command: [sh, ` + script + `]
    events: [lint.check]
lint:
  packs: [./lint/team]
`,
		"lint/team/pack.yaml": `name: team
rules:
  - id: team/no-todo
    engine: todo-lint
    severity: error
    scope: "src/**/*.ts"
    message: A TODO is left in the code.
`,
		todoSourceFile: "export const total = 1\n// TODO: add the tax\n",
	})
	want := shareddomain.Finding{Rule: todoRuleID, Severity: shareddomain.SeverityError, File: todoSourceFile, Line: 2, Message: "A TODO is left in the code."}

	if code, _, stderr := execute(t, commandLint, commandInit); code != exitOK {
		t.Fatalf("lint init: code = %d, stderr = %q", code, stderr)
	}

	code, stdout, stderr := execute(t, commandLint, flagLintJSON)
	var report shareddomain.Report
	if code != exitOK || json.Unmarshal([]byte(stdout), &report) != nil || !slices.Equal(report.Findings, []shareddomain.Finding{want}) {
		t.Fatalf("lint --json: code = %d, stdout = %q, stderr = %q, want the %s finding", code, stdout, stderr, todoRuleID)
	}

	code, stdout, _ = execute(t, commandLint)
	if code != exitRuntime || !strings.Contains(stdout, todoRuleID) || !strings.Contains(stdout, todoSourceFile) {
		t.Fatalf("lint: code = %d, stdout = %q, want exit %d and the finding", code, stdout, exitRuntime)
	}

	if code, _, stderr := execute(t, commandLint, commandBaseline); code != exitOK {
		t.Fatalf("lint baseline: code = %d, stderr = %q", code, stderr)
	}
	raw, err := os.ReadFile(filepath.Join(dir, lintBaselineFile))
	if err != nil {
		t.Fatal(err)
	}
	var baseline shareddomain.Report
	if err := json.Unmarshal(raw, &baseline); err != nil || !slices.Contains(baseline.Findings, want) {
		t.Fatalf("baseline = %s, %v, want the %s finding", raw, err, todoRuleID)
	}
}
