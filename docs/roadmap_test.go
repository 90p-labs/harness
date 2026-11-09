package docs_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	roadmapFile   = "ROADMAP.md"
	fixesCell     = 4
	noFixesValue  = "none"
	idSeparator   = ","
	cellSeparator = "|"
	kodifyIDWant  = 16
)

var (
	kodifyRow = regexp.MustCompile(`(?m)^\| (K\d+) \|`)
)

func TestEveryKodifyIssueHasARoadmapPR(t *testing.T) {
	design, err := os.ReadFile(repository.Path(t, designFile))
	if err != nil {
		t.Fatal(err)
	}
	ids := kodifyRow.FindAllStringSubmatch(string(design), -1)
	if len(ids) != kodifyIDWant {
		t.Fatalf("%s has %d kodify ids, want %d", designFile, len(ids), kodifyIDWant)
	}

	roadmap, err := os.ReadFile(repository.Path(t, roadmapFile))
	if err != nil {
		t.Fatal(err)
	}
	fixed := fixedIDs(string(roadmap))

	for _, id := range ids {
		if !fixed[id[1]] {
			t.Errorf("%s has no PR in the first table of %s", id[1], roadmapFile)
		}
	}
}

func fixedIDs(roadmap string) map[string]bool {
	fixed := map[string]bool{}
	inTable := false
	for _, line := range strings.Split(roadmap, "\n") {
		if !strings.HasPrefix(line, cellSeparator) {
			if inTable {
				break
			}
			continue
		}
		inTable = true
		cells := strings.Split(line, cellSeparator)
		if len(cells) <= fixesCell {
			continue
		}
		for _, id := range strings.Split(cells[fixesCell], idSeparator) {
			if id = strings.TrimSpace(id); id != noFixesValue {
				fixed[id] = true
			}
		}
	}
	return fixed
}
