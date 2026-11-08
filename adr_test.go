package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const adrDir = "docs/adr"

var (
	adrNumbers  = []string{"0001", "0002", "0003", "0004", "0005", "0006", "0007", "0008", "0009", "0010", "0012"}
	adrSections = []string{"## Status", "## Context", "## Decision", "## Consequences"}
)

func TestADRSections(t *testing.T) {
	for _, number := range adrNumbers {
		files, err := filepath.Glob(filepath.Join(adrDir, number+"-*.md"))
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
