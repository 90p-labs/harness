package harness

import (
	"slices"
	"strings"
	"testing"
)

const prOpenSkill = "skills/pr-open/SKILL.md"

var (
	prBodySections = []string{"## Why", "## What", "## How"}
	prOpenSteps    = []string{"harness run status", "harness diagram", "harness verify --commits", "gh pr create --draft", "gh pr edit", "--body-file"}
)

func TestPROpen(t *testing.T) {
	skill := readDoc(t, prOpenSkill)

	last := -1
	for _, section := range prBodySections {
		at := strings.Index(skill, "`"+section+"`")
		if at < 0 || at < last {
			t.Errorf("%s: the section %q is missing or out of order", prOpenSkill, section)
		}
		last = at
	}
	for _, step := range prOpenSteps {
		if !strings.Contains(skill, step) {
			t.Errorf("%s does not name %q", prOpenSkill, step)
		}
	}
	if keys := frontmatterKeys(t, prOpenSkill); !slices.Equal(keys, []string{"argument-hint", "description", "name"}) {
		t.Errorf("%s frontmatter keys = %v, want argument-hint, description and name", prOpenSkill, keys)
	}
}

func TestPROpenDocs(t *testing.T) {
	design, readme := readDoc(t, designFile), readDoc(t, readmeFile)
	for _, section := range prBodySections {
		if !strings.Contains(design, "`"+section+"`") {
			t.Errorf("%s does not name the PR body section %s", designFile, section)
		}
	}
	if strings.Contains(design, "`## Summary`, `## What`") {
		t.Errorf("%s still names the old PR body sections", designFile)
	}
	if !strings.Contains(readme, "`pr-open`") {
		t.Errorf("%s does not name the `pr-open` skill", readmeFile)
	}
}

func TestPROpenBodyFile(t *testing.T) {
	if skill := readDoc(t, prOpenSkill); strings.Contains(skill, "--body-file pr-body.md") {
		t.Errorf("%s passes a relative body file, which the shell does not find from the repository", prOpenSkill)
	}
}

func TestPROpenShipStep(t *testing.T) {
	if skill := readDoc(t, skillFile); !strings.Contains(skill, "`pr-open`") {
		t.Errorf("%s does not name the `pr-open` skill", skillFile)
	}
}
