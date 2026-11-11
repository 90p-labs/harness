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
	prCheckStep = "harness pr check --body-file"
	prBodyStep  = "harness pr body"
	prCreate    = "gh pr create"
)

var (
	prBodySections = []string{"## Why", "## What", "## How"}
	prOpenSteps    = []string{"harness run status", "harness diagram", "harness verify --commits", prBodyStep, prCheckStep, "gh pr create --draft", "gh pr edit", "--body-file"}
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
	if strings.Index(skill, prCheckStep) > strings.Index(skill, prCreate) {
		t.Errorf("%s runs %q after %q", prOpenSkill, prCheckStep, prCreate)
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
	if !strings.HasPrefix(strings.TrimSpace(above), "<bottom line") {
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

const policyCommand = "harness pr policy"

func TestPROpenPolicy(t *testing.T) {
	skill := repository.Read(t, prOpenSkill)

	policy, create := strings.Index(skill, policyCommand), strings.Index(skill, prCreate)
	if policy < 0 || policy > create {
		t.Errorf("%s must run %q before %q", prOpenSkill, policyCommand, prCreate)
	}
	for _, want := range []string{
		"draft: required", "draft: optional", "merge: user", "merge: agent",
		"gh pr create --draft", "gh pr ready <url>", "gh pr view --json url,state,isDraft",
		"gh pr merge <url> --squash", "gh pr checks <url>", "exits 0",
		"failed, pending or missing check means stop and report",
		"the guard denies `gh pr ready`",
		"If `harness pr policy` fails, keep the draft",
	} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q", prOpenSkill, want)
		}
	}
	if merge, report := strings.Index(skill, "gh pr merge"), strings.Index(skill, "harness run report"); report < 0 {
		t.Errorf("%s does not name %q", prOpenSkill, "harness run report")
	} else if merge > report {
		t.Errorf("%s merges after the report", prOpenSkill)
	}
	requiredLine, optionalLine, failedLine := "", "", ""
	requiredAgentLine, userLine, optionalAgentLine := "", "", ""
	for line := range strings.SplitSeq(skill, "\n") {
		switch trimmed := strings.TrimSpace(line); {
		case strings.HasPrefix(trimmed, "- `draft: required` with `merge: agent`"):
			requiredAgentLine = trimmed
		case strings.HasPrefix(trimmed, "- `merge: user`"):
			userLine = trimmed
		case strings.HasPrefix(trimmed, "- `draft: optional` with `merge: agent`"):
			optionalAgentLine = trimmed
		case strings.HasPrefix(trimmed, "- `draft: required`, or"):
			requiredLine = trimmed
		case strings.HasPrefix(trimmed, "- `draft: optional`:"):
			optionalLine = trimmed
		case strings.HasPrefix(trimmed, "- A failed `harness pr policy`"):
			failedLine = trimmed
		}
	}
	if !strings.Contains(requiredLine, "gh pr create --draft") {
		t.Errorf("%s: the `draft: required` line of step 4 must run %q, got %q", prOpenSkill, "gh pr create --draft", requiredLine)
	}
	if !strings.Contains(optionalLine, "without `--draft`") {
		t.Errorf("%s: the `draft: optional` line of step 4 must run the command %q, got %q", prOpenSkill, "without `--draft`", optionalLine)
	}
	_, step7, _ := strings.Cut(skill, "\n7. ")
	step7, _, _ = strings.Cut(step7, "\n")
	optionalSentence, requiredSentence, _ := strings.Cut(step7, ". ")
	if !strings.HasPrefix(optionalSentence, "With `draft: optional`") || !strings.Contains(optionalSentence, "run `gh pr ready <url>`") {
		t.Errorf("%s: the `draft: optional` sentence of step 7 must run %q, got %q", prOpenSkill, "gh pr ready <url>", optionalSentence)
	}
	if !strings.HasPrefix(requiredSentence, "With `draft: required`") || !strings.Contains(requiredSentence, "leave the draft") || strings.Contains(requiredSentence, "gh pr ready") {
		t.Errorf("%s: the `draft: required` sentence of step 7 must leave the draft and not run %q, got %q", prOpenSkill, "gh pr ready", requiredSentence)
	}
	if !strings.Contains(failedLine, "report the URL") || !strings.Contains(strings.ToLower(failedLine), "the user merges") {
		t.Errorf("%s: step 4.8 needs a failed policy bullet that reports the URL and leaves the merge to the user, got %q", prOpenSkill, failedLine)
	}
	if strings.Contains(failedLine, "gh pr merge") {
		t.Errorf("%s: the failed policy bullet of step 8 must not run %q, got %q", prOpenSkill, "gh pr merge", failedLine)
	}
	if !strings.Contains(userLine, "report the URL") || !strings.Contains(userLine, "The user merges") || strings.Contains(userLine, "gh pr merge") {
		t.Errorf("%s: the `merge: user` bullet of step 8 must report the URL, say %q and not run %q, got %q", prOpenSkill, "The user merges", "gh pr merge", userLine)
	}
	if !strings.Contains(requiredAgentLine, "report the URL") || !strings.Contains(requiredAgentLine, "The user marks it ready and merges") ||
		strings.Contains(requiredAgentLine, "gh pr merge") || strings.Contains(strings.ReplaceAll(requiredAgentLine, "the guard denies `gh pr ready`", ""), "gh pr ready") {
		t.Errorf("%s: the `draft: required` with `merge: agent` bullet of step 8 must leave the ready and the merge to the user and run neither %q nor %q, got %q", prOpenSkill, "gh pr merge", "gh pr ready", requiredAgentLine)
	}
	checks, merge := strings.Index(optionalAgentLine, "gh pr checks <url>"), strings.Index(optionalAgentLine, "gh pr merge <url> --squash")
	gate := strings.Index(optionalAgentLine, "Only when it exits 0, run `gh pr merge <url> --squash`")
	if checks < 0 || merge < checks || gate < checks {
		t.Errorf("%s: the `draft: optional` with `merge: agent` bullet of step 8 must run %q, then %q only when it exits 0, got %q", prOpenSkill, "gh pr checks <url>", "gh pr merge <url> --squash", optionalAgentLine)
	}
	if !strings.Contains(optionalAgentLine, "failed, pending or missing check means stop and report") {
		t.Errorf("%s: the `draft: optional` with `merge: agent` bullet of step 8 must say %q, got %q", prOpenSkill, "failed, pending or missing check means stop and report", optionalAgentLine)
	}
	if n := strings.Count(skill, "gh pr merge"); n != 1 {
		t.Errorf("%s names %q %d times; only the `draft: optional` with `merge: agent` bullet of step 8 may merge", prOpenSkill, "gh pr merge", n)
	}
	for _, old := range []string{"Do not mark the pull request ready and do not merge it", "branch as a draft, or edits"} {
		if strings.Contains(skill, old) {
			t.Errorf("%s still says %q", prOpenSkill, old)
		}
	}
	if strings.Contains(skill, "--delete-branch") {
		t.Errorf("%s names --delete-branch, which fails in a linked worktree", prOpenSkill)
	}
}

func TestPRPolicyDocs(t *testing.T) {
	for _, path := range []string{readmeFile, designFile, "docs/configuration.md"} {
		if !strings.Contains(repository.Read(t, path), policyCommand) {
			t.Errorf("%s does not name %q", path, policyCommand)
		}
	}
	for _, path := range []string{designFile, "docs/configuration.md"} {
		for line := range strings.SplitSeq(repository.Read(t, path), "\n") {
			if strings.HasPrefix(line, "| `pr-draft-only`") && !strings.Contains(line, "gh pr ready") {
				t.Errorf("%s: the pr-draft-only row does not name `gh pr ready`", path)
			}
		}
	}
	if !strings.Contains(repository.Read(t, designFile), "`pr body`, `pr policy` and `report` load the config") {
		t.Errorf("%s: the config loaders sentence does not name `pr policy`", designFile)
	}
	if readme := repository.Read(t, readmeFile); strings.Contains(readme, "opens a draft pull request and stops") {
		t.Errorf("%s still says the agent always opens a draft and stops", readmeFile)
	}
}

func TestPROpenShipStep(t *testing.T) {
	if skill := repository.Read(t, skillFile); !strings.Contains(skill, "`pr-open`") {
		t.Errorf("%s does not name the `pr-open` skill", skillFile)
	}
}
