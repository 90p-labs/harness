package lintsetup_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/lint/infrastructure/rulecatalogue"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	lintSetupSkill = "skills/lint-setup/SKILL.md"
	proofCommand   = "harness lint canaries --proof"
	proofFolder    = "lint/proof/"
)

var (
	committedProofPaths = []string{"lint/rules.yaml", "lint/canaries"}
	knownLintTools      = []string{"deptrac", "import-linter", "go-arch-lint", "depguard", "`ddd` pack"}
)

func requireProof(t *testing.T, path, text string) {
	t.Helper()
	for _, want := range []string{proofCommand, proofFolder} {
		if !strings.Contains(text, want) {
			t.Errorf("%s does not name %q", path, want)
		}
	}
	for _, refused := range committedProofPaths {
		if strings.Contains(text, refused) {
			t.Errorf("%s names %q, which a project no longer commits", path, refused)
		}
	}
}

func TestLintSetupSkill(t *testing.T) {
	skill := repository.Read(t, lintSetupSkill)
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
	requireProof(t, lintSetupSkill, skill)
	if keys := repository.FrontmatterKeys(t, lintSetupSkill); !slices.Contains(keys, "argument-hint") {
		t.Errorf("%s frontmatter keys = %v, want argument-hint for the language", lintSetupSkill, keys)
	}
}
