package harness

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	importLinterPlugin = "examples/python/.harness/plugins/import-linter.sh"
	fakeDocker         = "#!/bin/sh\ncat \"$FAKE_LINT_IMPORTS\"\nexit 1\n"
	fakeReportEnv      = "FAKE_LINT_IMPORTS"
	ruleSrcRoots       = "ddd/src-roots"
	ruleDomainPurity   = "ddd/domain-purity"
	ruleLayerDirection = "ddd/layer-direction"
	ruleCrossContext   = "ddd/cross-context"
	renamedContract    = "Contexts are independent"
)

const brokenReport = `
---------
Contracts
---------

Analyzed 27 files, 12 dependencies.
-----------------------------------

ddd/src-roots BROKEN
ddd/src-roots KEPT
ddd/domain-purity BROKEN
ddd/layer-direction BROKEN
ddd/cross-context KEPT (2 ignored imports)
ddd/cross-context BROKEN
shop/no-requests BROKEN

Contracts: 2 kept, 5 broken.


----------------
Broken contracts
----------------

ddd/src-roots
-------------

The following modules are not listed as layers:

- shop.cart
- shop.payment

(Since this contract is marked as 'exhaustive', every child of every container must be declared as a layer.)

ddd/domain-purity
-----------------

shop.billing.domain is not allowed to import requests:

-   shop.billing.domain.invoice -> requests (l.5)

shop.billing.domain is not allowed to import shop.billing.infrastructure.gateway:

-   shop.billing.domain.invoice -> shop.billing.infrastructure.gateway (l.3)

shop.shared.domain is not allowed to import shop.order:

-   shop.shared.domain.bad -> shop.order (l.1)

ddd/layer-direction
-------------------

shop.order.application is not allowed to import shop.order.infrastructure:

- shop.order.application.direct -> shop.order.infrastructure.memory_repository (l.1)
- shop.order.application.wire -> shop.order (l.1)
  shop.order -> shop.order.infrastructure.memory_repository (l.1)


shop.billing.application is not allowed to import shop.billing.infrastructure:

- shop.billing.application.issue_invoice -> shop.billing.domain.invoice (l.1)
  shop.billing.domain.invoice -> shop.billing.infrastructure.gateway (l.3)


shop.shared.application is not allowed to import shop.shared.infrastructure:

- shop.shared.application.rewire (l.2)
  & shop.shared.application.wire -> shop.shared (l.4, l.6)
  shop.shared -> shop.shared.infrastructure.store (l.1)
                 & shop.shared.infrastructure.other (l.2)


ddd/cross-context
-----------------

shop.shared is not allowed to import shop.order:

-   shop.shared.domain.bad -> shop.order (l.1)


shop/no-requests
----------------

shop.billing is not allowed to import requests:

-   shop.billing.domain.invoice -> requests (l.5)
`

const renamedReport = `
---------
Contracts
---------

Analyzed 27 files, 12 dependencies.
-----------------------------------

ddd/src-roots KEPT
ddd/src-roots KEPT
ddd/domain-purity KEPT
ddd/layer-direction KEPT
Contexts are independent KEPT (2 ignored imports)
ddd/cross-context KEPT

Contracts: 6 kept, 0 broken.
`

var payloadFiles = []string{
	"src/shop/__init__.py",
	"src/shop/cart.py",
	"src/shop/payment/__init__.py",
	"src/shop/billing/domain/invoice.py",
	"src/shop/billing/application/issue_invoice.py",
	"src/shop/order/application/direct.py",
	"src/shop/order/application/wire.py",
	"src/shop/shared/domain/bad.py",
	"src/shop/shared/application/rewire.py",
	"src/shop/shared/application/wire.py",
	"src/shop/shared/infrastructure/other.py",
}

var allRules = []string{ruleSrcRoots, ruleDomainPurity, ruleLayerDirection, ruleCrossContext}

type pluginFinding struct {
	Rule string `json:"rule"`
	File string `json:"file"`
	Line int    `json:"line"`
}

func TestImportLinterPlugin(t *testing.T) {
	t.Run("one finding for each import that breaks a contract, under the rule of its contract", func(t *testing.T) {
		got := brokenReportFindings(t, allRules)
		want := []pluginFinding{
			{Rule: ruleCrossContext, File: "src/shop/shared/domain/bad.py", Line: 1},
			{Rule: ruleDomainPurity, File: "src/shop/billing/domain/invoice.py", Line: 3},
			{Rule: ruleDomainPurity, File: "src/shop/billing/domain/invoice.py", Line: 5},
			{Rule: ruleDomainPurity, File: "src/shop/shared/domain/bad.py", Line: 1},
			{Rule: ruleLayerDirection, File: "src/shop/order/application/direct.py", Line: 1},
			{Rule: ruleLayerDirection, File: "src/shop/order/application/wire.py", Line: 1},
			{Rule: ruleLayerDirection, File: "src/shop/shared/application/rewire.py", Line: 2},
			{Rule: ruleLayerDirection, File: "src/shop/shared/application/wire.py", Line: 4},
			{Rule: ruleSrcRoots, File: "src/shop/cart.py", Line: 1},
			{Rule: ruleSrcRoots, File: "src/shop/payment/__init__.py", Line: 1},
		}
		if !slices.Equal(got, want) {
			t.Errorf("findings = %+v\nwant %+v", got, want)
		}
	})
	t.Run("a chain drops only on a direct import of a rule in the payload", func(t *testing.T) {
		got := brokenReportFindings(t, []string{ruleLayerDirection})
		want := []pluginFinding{
			{Rule: ruleLayerDirection, File: "src/shop/billing/application/issue_invoice.py", Line: 1},
			{Rule: ruleLayerDirection, File: "src/shop/order/application/direct.py", Line: 1},
			{Rule: ruleLayerDirection, File: "src/shop/order/application/wire.py", Line: 1},
			{Rule: ruleLayerDirection, File: "src/shop/shared/application/rewire.py", Line: 2},
			{Rule: ruleLayerDirection, File: "src/shop/shared/application/wire.py", Line: 4},
		}
		if !slices.Equal(got, want) {
			t.Errorf("findings = %+v\nwant %+v", got, want)
		}
	})
	t.Run("a contract name that is not a rule id is a plugin error", func(t *testing.T) {
		code, _, stderr := runImportLinterPlugin(t, renamedReport, allRules)
		if code == 0 || !strings.Contains(stderr, renamedContract) {
			t.Errorf("exit %d, stderr %q, want a failure that names %q", code, stderr, renamedContract)
		}
	})
}

func brokenReportFindings(t *testing.T, rules []string) []pluginFinding {
	t.Helper()
	code, stdout, stderr := runImportLinterPlugin(t, brokenReport, rules)
	if code != 0 {
		t.Fatalf("exit %d, stderr %s", code, stderr)
	}
	var verdict struct {
		Findings []pluginFinding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(stdout), &verdict); err != nil {
		t.Fatalf("stdout %q: %v", stdout, err)
	}
	return verdict.Findings
}

func runImportLinterPlugin(t *testing.T, report string, rules []string) (int, string, string) {
	t.Helper()
	plugin, err := filepath.Abs(importLinterPlugin)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(fakeDocker), executableMode); err != nil {
		t.Fatal(err)
	}
	reportFile := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(reportFile, []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	payloadRules := make([]map[string]string, len(rules))
	for i, rule := range rules {
		payloadRules[i] = map[string]string{"id": rule}
	}
	envelope, err := json.Marshal(map[string]any{"payload": map[string]any{
		"rules": payloadRules,
		"files": payloadFiles,
	}})
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", plugin)
	cmd.Dir = gitRepo(t, false)
	cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), fakeReportEnv+"="+reportFile)
	cmd.Stdin = bytes.NewReader(envelope)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, stdout.String(), stderr.String()
}
