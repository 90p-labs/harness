package harness

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/lint/infrastructure/rulecatalogue"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	lintSetupSkill = "skills/lint-setup/SKILL.md"
	linterAgent    = "agents/linter.md"
	reviewerAgent  = "agents/reviewer.md"
	frontmatterEnd = "\n---\n"

	initLintSetupLine = "next: run the lint-setup skill"
)

var knownLintTools = []string{"deptrac", "import-linter", "go-arch-lint", "depguard", "`ddd` pack"}

func frontmatterKeys(t *testing.T, path string) []string {
	t.Helper()
	head, _, found := strings.Cut(strings.TrimPrefix(readDoc(t, path), "---\n"), frontmatterEnd)
	var fields map[string]any
	if !found || yaml.Unmarshal([]byte(head), &fields) != nil {
		t.Fatalf("%s: no YAML frontmatter", path)
	}
	return slices.Sorted(maps.Keys(fields))
}

func TestLintSetupSkill(t *testing.T) {
	skill := readDoc(t, lintSetupSkill)
	catalogue, err := rulecatalogue.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range catalogue.Packs {
		for _, rule := range pack.Rules {
			if !strings.Contains(skill, "`"+rule.ID+"`") {
				t.Errorf("%s does not name the catalogue rule %s", lintSetupSkill, rule.ID)
			}
		}
	}
	for _, tool := range knownLintTools {
		if !strings.Contains(skill, tool) {
			t.Errorf("%s does not name the known tool %s", lintSetupSkill, tool)
		}
	}
	wants := []string{shareddomain.RecommendedSuffix[1:]}
	for _, tool := range shareddomain.QuestionTools {
		wants = append(wants, tool.Name)
	}
	for _, want := range wants {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q for its questions", lintSetupSkill, want)
		}
	}
	if keys := frontmatterKeys(t, lintSetupSkill); !slices.Contains(keys, "argument-hint") {
		t.Errorf("%s frontmatter keys = %v, want argument-hint for the language", lintSetupSkill, keys)
	}
}

func TestLintSetupStartsAfterInit(t *testing.T) {
	skill := readDoc(t, skillFile)

	for _, want := range []string{initLintSetupLine, "`lint-setup`"} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q", skillFile, want)
		}
	}
}

func TestLintSetupAgent(t *testing.T) {
	got, want := frontmatterKeys(t, linterAgent), frontmatterKeys(t, reviewerAgent)

	if !slices.Equal(got, want) {
		t.Errorf("%s frontmatter keys = %v, want the keys of %s: %v", linterAgent, got, reviewerAgent, want)
	}
}
