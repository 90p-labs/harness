package propen_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	skillFile   = "skills/harness/SKILL.md"
	designFile  = "DESIGN.md"
	readmeFile  = "README.md"
	prOpenSkill = "skills/pr-open/SKILL.md"
)

var (
	prBodySections = []string{"## Why", "## What", "## How"}
	prOpenSteps    = []string{"harness run status", "harness diagram", "harness verify --commits", "gh pr create --draft", "gh pr edit", "--body-file"}
)

func TestPROpen(t *testing.T) {
	skill := repository.Read(t, prOpenSkill)

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
	if keys := repository.FrontmatterKeys(t, prOpenSkill); !slices.Equal(keys, []string{"argument-hint", "description", "name"}) {
		t.Errorf("%s frontmatter keys = %v, want argument-hint, description and name", prOpenSkill, keys)
	}
}

const prVisualsRef = "skills/pr-open/references/visuals.md"

var (
	prMermaidKinds = []string{"stateDiagram-v2", "flowchart", "sequenceDiagram", "journey", "classDiagram", "erDiagram"}
	prWireframe    = "wireframe"
)

func fenceFirstLines(doc, lang string) []string {
	var firsts []string
	parts := strings.Split(doc, "```"+lang+"\n")
	for _, part := range parts[1:] {
		firsts = append(firsts, strings.SplitN(part, "\n", 2)[0])
	}
	return firsts
}

func TestPROpenVisuals(t *testing.T) {
	skill, ref := repository.Read(t, prOpenSkill), repository.Read(t, prVisualsRef)
	for _, kind := range append([]string{prWireframe}, prMermaidKinds...) {
		if !strings.Contains(skill, kind) {
			t.Errorf("%s does not name the visual %q", prOpenSkill, kind)
		}
	}
	if !strings.Contains(skill, "references/visuals.md") {
		t.Errorf("%s does not link references/visuals.md", prOpenSkill)
	}
	firsts := fenceFirstLines(ref, "mermaid")
	for _, kind := range prMermaidKinds {
		n := 0
		for _, first := range firsts {
			if strings.HasPrefix(first, kind) {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s has %d mermaid fences of kind %s, want 1", prVisualsRef, n, kind)
		}
	}
	if n := len(fenceFirstLines(ref, "text")); n != 1 {
		t.Errorf("%s has %d text fences, want 1 wireframe", prVisualsRef, n)
	}
}

func TestPROpenBottomLine(t *testing.T) {
	skill := repository.Read(t, prOpenSkill)
	if !strings.Contains(skill, "BLUF") {
		t.Errorf("%s does not name BLUF", prOpenSkill)
	}
	why := strings.Index(skill, "\n## Why\n")
	if why < 0 {
		t.Fatalf("%s has no body template", prOpenSkill)
	}
	above := skill[strings.LastIndex(skill[:why], "```\n")+4 : why]
	if !strings.HasPrefix(strings.TrimSpace(above), "<the bottom line") {
		t.Errorf("%s: the body template has no bottom line placeholder above ## Why", prOpenSkill)
	}
}

func TestPROpenDocs(t *testing.T) {
	design, readme := repository.Read(t, designFile), repository.Read(t, readmeFile)
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
	if skill := repository.Read(t, prOpenSkill); strings.Contains(skill, "--body-file pr-body.md") {
		t.Errorf("%s passes a relative body file, which the shell does not find from the repository", prOpenSkill)
	}
}

func TestPROpenShipStep(t *testing.T) {
	if skill := repository.Read(t, skillFile); !strings.Contains(skill, "`pr-open`") {
		t.Errorf("%s does not name the `pr-open` skill", skillFile)
	}
}
