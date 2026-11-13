package docs_test

import (
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	learnADR       = "docs/adr/0024-learn-conventions-at-ship.md"
	learnIndex     = "docs/adr/index.md"
	learnIndexRow  = "| [0024](0024-learn-conventions-at-ship.md) | 0024 Learn conventions at ship |"
	learnCommand   = "harness run learn"
	learnSpan      = "run.learn"
	learnReadme    = "README.md"
	learnDesign    = "DESIGN.md"
	learnConfigDoc = "docs/configuration.md"
	learnAudit     = "`harness run learn [<run id>]` prints"
)

var learnDecisions = []string{
	"read-only query", "`run.learn`", "`pr-open`", "lens `standards`", "`gate-checkable` rework",
	"source `user`", "`Conventions`", "`harness run start` prompt", "`docs/adr/index.md`", "`CONTEXT.md`",
}

func TestRunsPageShowsTheShippingHint(t *testing.T) {
	const stale = "next: open the pull request and run harness run wait"
	const current = "next: run harness run learn, open the pull request and run harness run wait --pr URL, then after the merge harness run done --pr URL"
	page := repository.Read(t, "docs/runs.md")
	if strings.Contains(page, stale) {
		t.Errorf("docs/runs.md shows the shipping hint with no learn step: %q", stale)
	}
	if !strings.Contains(page, current) {
		t.Errorf("docs/runs.md does not show the shipping hint %q", current)
	}
}

func TestDocsNameTheLearnStep(t *testing.T) {
	adr := repository.Read(t, learnADR)
	for _, want := range learnDecisions {
		if !strings.Contains(adr, want) {
			t.Errorf("%s does not record %q", learnADR, want)
		}
	}
	if !strings.Contains(repository.Read(t, learnConfigDoc), learnAudit) {
		t.Errorf("%s does not describe %q", learnConfigDoc, learnAudit)
	}

	index := repository.Read(t, learnIndex)
	if !strings.Contains(index, learnIndexRow) {
		t.Errorf("%s has no row %q", learnIndex, learnIndexRow)
	}

	for _, doc := range []string{learnReadme, learnDesign, learnConfigDoc} {
		if !strings.Contains(repository.Read(t, doc), learnCommand) {
			t.Errorf("%s does not name %q", doc, learnCommand)
		}
	}

	for _, doc := range []string{learnDesign, learnConfigDoc} {
		if !strings.Contains(repository.Read(t, doc), learnSpan) {
			t.Errorf("%s does not name the span %q", doc, learnSpan)
		}
	}
}
