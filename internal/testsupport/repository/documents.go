package repository

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/90p-labs/harness/internal/shared/domain"
)

const EmDash = "—"

// Read reads a document from its repository-relative or absolute asset path.
func Read(t testing.TB, path string) string {
	t.Helper()
	raw, err := os.ReadFile(Path(t, path))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// Section returns the body of a Markdown section, up to the next level-two heading.
func Section(t testing.TB, doc, heading string) string {
	t.Helper()
	_, rest, found := strings.Cut(doc, heading)
	if !found {
		t.Fatalf("no section %q", strings.TrimSpace(heading))
	}
	body, _, _ := strings.Cut(rest, "\n## ")
	return body
}

// FrontmatterKeys returns sorted YAML frontmatter keys for a skill or agent document.
func FrontmatterKeys(t testing.TB, path string) []string {
	t.Helper()
	head, _, found := strings.Cut(strings.TrimPrefix(Read(t, path), "---\n"), "\n---\n")
	var fields map[string]any
	if !found || yaml.Unmarshal([]byte(head), &fields) != nil {
		t.Fatalf("%s: no YAML frontmatter", path)
	}
	return slices.Sorted(maps.Keys(fields))
}

// IsEmoji matches the symbol ranges disallowed in project documentation.
func IsEmoji(r rune) bool {
	return domain.IsEmoji(r)
}
