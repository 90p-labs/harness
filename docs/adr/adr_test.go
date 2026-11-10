package adr_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	adrDir     = "docs/adr"
	adrPattern = "[0-9][0-9][0-9][0-9]-*.md"
	indexFile  = "docs/adr/index.md"
)

var (
	adrSections = []string{"## Status", "## Context", "## Decision", "## Consequences"}
	indexLink   = regexp.MustCompile(`\((\d{4}-[^)\s]+\.md)\)`)
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

// adrFiles returns the ADR files in the folder, found by glob.
func adrFiles(t *testing.T) []string {
	t.Helper()
	files, err := filepath.Glob(repository.Path(t, adrDir, adrPattern))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatalf("no ADR file in %s", adrDir)
	}
	return files
}

func TestADRSections(t *testing.T) {
	for _, file := range adrFiles(t) {
		number := filepath.Base(file)[:4]
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(raw)
		if !strings.HasPrefix(text, "# "+number+" ") {
			t.Errorf("%s does not start with %q", file, "# "+number+" ")
		}
		for _, section := range adrSections {
			if !strings.Contains(text, "\n"+section+"\n") {
				t.Errorf("%s misses the section %q", file, section)
			}
		}
	}
}

func TestADRNumbersAreUnique(t *testing.T) {
	seen := map[string]string{}
	for _, file := range adrFiles(t) {
		name := filepath.Base(file)
		number := name[:4]
		if other, ok := seen[number]; ok {
			t.Errorf("%s and %s share the ADR number %s", other, name, number)
		}
		seen[number] = name
	}
}

func TestADRIndexListsEveryADR(t *testing.T) {
	index := repository.Read(t, indexFile)
	lines := strings.Split(index, "\n")
	for _, file := range adrFiles(t) {
		name := filepath.Base(file)
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		title := ""
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.HasPrefix(line, "# ") {
				title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
				break
			}
		}
		if title == "" {
			t.Errorf("%s has no title line", file)
			continue
		}
		found := false
		for _, line := range lines {
			if strings.Contains(line, "("+name+")") && strings.Contains(line, title) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("%s has no entry for %q with the title %q", indexFile, name, title)
		}
	}
	for _, m := range indexLink.FindAllStringSubmatch(index, -1) {
		if _, err := os.Stat(repository.Path(t, adrDir, m[1])); err != nil {
			t.Errorf("%s names %s, which does not exist", indexFile, m[1])
		}
	}
}
