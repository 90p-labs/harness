package docs_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
	rundomain "github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	bugADR       = "docs/adr/0025-reproduce-a-bug-before-the-plan.md"
	bugIndexRow  = "| [0025](0025-reproduce-a-bug-before-the-plan.md) | 0025 Reproduce a bug before the plan |"
	bugTermEntry = "- **Reproduction**:"
)

func TestBugPathDocs(t *testing.T) {
	tags := []string{
		domain.CausePrefix + " " + domain.CauseBug, "## " + domain.HeadingReproduction,
		"`" + string(domain.RuleReproduction) + "`", "`" + string(domain.RuleReproductionRed) + "`",
		"harness verify --criterion", string(rundomain.SourceNoSeam),
	}
	for _, doc := range []string{"DESIGN.md", "docs/runs.md", bugADR} {
		text := repository.Read(t, doc)
		for _, want := range tags {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not name %q", doc, want)
			}
		}
		if strings.Contains(text, repository.EmDash) {
			t.Errorf("%s has an em dash", doc)
		}
	}

	adr := repository.Read(t, bugADR)
	for _, want := range []string{
		"## Status", "## Context", "## Decision", "## Consequences", "Matt Pocock", "diagnosing-bugs", "MIT",
		domain.ReproHumanOnly, domain.ReproRegression, "`worker` agent", "`planning`",
	} {
		if !strings.Contains(adr, want) {
			t.Errorf("%s does not record %q", bugADR, want)
		}
	}
	for n := 2; n <= 8; n++ {
		if !strings.Contains(adr, fmt.Sprintf("\n%d. ", n)) {
			t.Errorf("%s has no decision item %d at the start of a line", bugADR, n)
		}
	}
	if !strings.Contains(repository.Read(t, "docs/adr/index.md"), bugIndexRow) {
		t.Errorf("docs/adr/index.md has no row %q", bugIndexRow)
	}

	context := repository.Read(t, "CONTEXT.md")
	if !strings.Contains(context, bugTermEntry) {
		t.Errorf("CONTEXT.md has no entry %q", bugTermEntry)
	}
	_, entry, _ := strings.Cut(context, bugTermEntry)
	entry, _, _ = strings.Cut(entry, "\n")
	for _, want := range []string{"`" + domain.CausePrefix + " " + domain.CauseBug + "`", "`" + domain.ReproCriterion + "`", "`" + domain.ReproSymptom + "`"} {
		if !strings.Contains(entry, want) {
			t.Errorf("the Reproduction entry of CONTEXT.md does not name %s", want)
		}
	}
}
