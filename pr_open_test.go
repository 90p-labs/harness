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
		if at < last {
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

func TestPROpenShipStep(t *testing.T) {
	if skill := readDoc(t, skillFile); !strings.Contains(skill, "`pr-open`") {
		t.Errorf("%s does not name the `pr-open` skill", skillFile)
	}
}
