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
	commandCanaries  = "canaries"
	todoLintScript   = "testdata/todo-lint.sh"
	canaryLintScript = "testdata/canary-lint.sh"
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

const canaryManifest = `pack: ddd
rules:
  - id: ddd/domain-purity
    canary: lint/canaries/ddd-domain-purity
  - id: ddd/layer-direction
    canary: lint/canaries/ddd-layer-direction
  - id: ddd/file-suffix
    waived: the tool checks dependencies, not file names.
`

func TestLintCanaries(t *testing.T) {
	clearHarnessEnv(t)
	script, err := filepath.Abs(canaryLintScript)
	if err != nil {
		t.Fatal(err)
	}
	dir := initRepo(t)

	code, _, stderr := execute(t, commandLint, commandCanaries)
	if code != exitRuntime || !strings.Contains(stderr, lintdomain.ManifestFile) || !strings.Contains(stderr, "lint-setup") {
		t.Fatalf("no manifest: code = %d, stderr = %q, want exit %d naming %s and the lint-setup skill", code, stderr, exitRuntime, lintdomain.ManifestFile)
	}

	commitFiles(t, dir, "chore: canary repo", map[string]string{
		".harness.yaml": `plugins:
  - name: canary-lint
    command: [sh, ` + script + `]
    events: [lint.check]
lint:
  packs: [./lint/architecture]
`,
		"lint/architecture/pack.yaml": `name: architecture
rules:
  - id: ddd/domain-purity
    engine: canary-lint
    severity: error
    scope: "src/**/*.php"
    message: The domain depends only on itself.
  - id: ddd/layer-direction
    engine: canary-lint
    severity: error
    scope: "src/**/*.php"
    message: The application never depends on the infrastructure.
`,
		lintdomain.ManifestFile: canaryManifest,
		"lint/canaries/ddd-domain-purity/src/Order/Domain/Order.php":             "<?php\n// violates: ddd/domain-purity\n",
		"lint/canaries/ddd-layer-direction/src/Order/Application/PlaceOrder.php": "<?php\n// a canary that the engine misses\n",
	})

	code, _, stderr = execute(t, commandLint, commandCanaries)
	if code != exitRuntime || !strings.Contains(stderr, lintdomain.ErrStaleLintFiles.Error()) {
		t.Fatalf("no lint init: code = %d, stderr = %q, want exit %d and the stale lint files error", code, stderr, exitRuntime)
	}
	if code, _, stderr := execute(t, commandLint, commandInit); code != exitOK {
		t.Fatalf("lint init: code = %d, stderr = %q", code, stderr)
	}

	code, stdout, stderr := execute(t, commandLint, commandCanaries)

	want := "FAIL ddd/src-roots: not in lint/rules.yaml\n" +
		"PASS ddd/domain-purity lint/canaries/ddd-domain-purity\n" +
		"FAIL ddd/layer-direction lint/canaries/ddd-layer-direction: the engine canary-lint gave no ddd/layer-direction finding\n" +
		"FAIL ddd/cross-context: not in lint/rules.yaml\n" +
		"WAIVED ddd/file-suffix: the tool checks dependencies, not file names.\n"
	if code != exitRuntime || stdout != want {
		t.Fatalf("code = %d, stdout = %q, stderr = %q, want exit %d and %q", code, stdout, stderr, exitRuntime, want)
	}

	writeRepoFile(t, dir, "lint/canaries/ddd-layer-direction/src/Order/Application/PlaceOrder.php", "<?php\n// violates: ddd/layer-direction\n")
	writeRepoFile(t, dir, lintdomain.ManifestFile, canaryManifest+"  - id: ddd/cross-context\n    waived: the tool cannot see the contexts.\n  - id: ddd/src-roots\n    waived: the test pack places no file.\n")

	code, stdout, stderr = execute(t, commandLint, commandCanaries)

	want = "WAIVED ddd/src-roots: the test pack places no file.\n" +
		"PASS ddd/domain-purity lint/canaries/ddd-domain-purity\n" +
		"PASS ddd/layer-direction lint/canaries/ddd-layer-direction\n" +
		"WAIVED ddd/cross-context: the tool cannot see the contexts.\n" +
		"WAIVED ddd/file-suffix: the tool checks dependencies, not file names.\n"
	if code != exitOK || stdout != want {
		t.Fatalf("waived: code = %d, stdout = %q, stderr = %q, want exit %d and %q", code, stdout, stderr, exitOK, want)
	}

	writeRepoFile(t, dir, configFile, "lint:\n  packs: [./lint/architecture]\n")

	code, _, stderr = execute(t, commandLint, commandCanaries)

	if code != exitRuntime || !strings.Contains(stderr, "no lint plugin has that name") {
		t.Fatalf("no plugin: code = %d, stderr = %q, want exit %d and the unknown engine error", code, stderr, exitRuntime)
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
