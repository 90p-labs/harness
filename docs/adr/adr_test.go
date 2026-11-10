package adr_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	adrDir = "docs/adr"
)

var (
	adrNumbers  = []string{"0001", "0002", "0003", "0004", "0005", "0006", "0007", "0008", "0009", "0010", "0011", "0012", "0013", "0014", "0015", "0016", "0017", "0018"}
	adrSections = []string{"## Status", "## Context", "## Decision", "## Consequences"}
)

const (
	adrReviewRound = "docs/adr/0018-review-round-in-four-steps.md"
	contextFile    = "CONTEXT.md"
	oldPanelTerm   = "pre-ship panel"
	designFile     = "DESIGN.md"
	runsDoc        = "docs/runs.md"
	configDoc      = "docs/configuration.md"
)

var (
	reviewRoundSteps = []string{"**Common list**", "**Verify**", "**Fix**", "**Re-review**", "**Final panel**"}
	contextTerms     = []string{"**Common list**", "**Verifier**", "**Re-review**", "**Final panel**"}
	currentDocs      = []string{contextFile, designFile, runsDoc, configDoc}
)

func TestReviewRoundDocs(t *testing.T) {
	adr := repository.Read(t, adrReviewRound)
	for _, step := range reviewRoundSteps {
		if !strings.Contains(adr, step) {
			t.Errorf("%s misses the step %q", adrReviewRound, step)
		}
	}
	glossary := repository.Read(t, contextFile)
	for _, term := range contextTerms {
		if !strings.Contains(glossary, term) {
			t.Errorf("%s misses the term %q", contextFile, term)
		}
	}
	for _, doc := range currentDocs {
		if strings.Contains(strings.ToLower(repository.Read(t, doc)), oldPanelTerm) {
			t.Errorf("%s still holds %q", doc, oldPanelTerm)
		}
	}
}

func TestADRSections(t *testing.T) {
	for _, number := range adrNumbers {
		files, err := filepath.Glob(repository.Path(t, adrDir, number+"-*.md"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 {
			t.Errorf("ADR %s has %d files, want 1", number, len(files))
			continue
		}
		raw, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.HasPrefix(text, "# "+number+" ") {
			t.Errorf("%s does not start with %q", files[0], "# "+number+" ")
		}
		for _, section := range adrSections {
			if !strings.Contains(text, "\n"+section+"\n") {
				t.Errorf("%s misses the section %q", files[0], section)
			}
		}
	}
}
