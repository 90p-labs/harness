package agents_test

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	advisorFile   = "agents/advisor.md"
	workerFile    = "agents/worker.md"
	agentDocs     = "agents/*.md"
	reviewerAgent = "agents/reviewer.md"
	verifierAgent = "agents/verifier.md"
	findingLine   = "`<id> | <severity> | <file>:<line> | <defect and failure scenario> | <observed or inferred> | <proof>`"
	verifierLine  = "`<id> | real or not real | <severity> | <reason>`"
	filesReadLine = "Files read:"
	reReviewTitle = "## Re-review"
	standardsRule = "does not report a rule that a gate checks"
	reviewAnswer  = "Answer `closed` or `open`, to say whether its fixup fixes it."
	explorerFile  = "agents/explorer.md"
	templateName  = "references/plan-template.md"
	templateRoot  = "skills/harness/"
	binaryName    = "harness"
	helpFlag      = "--help"
	workerFixLine = "For a fix, the lead gives finding lines in place of a task. Your scope is the file of each finding and its test."
)

var (
	reportLimit    = regexp.MustCompile(`\b(?:under|at most) \d+ words\b`)
	harnessCommand = regexp.MustCompile("`harness ([a-z]+(?: [a-z]+)?)[^`]*`")
	helpCommand    = regexp.MustCompile(`(?m)^  ([a-z]+)\s`)
	explorerSteps  = []string{templateName, "git log --format='%h %s' -- <path>", "read-only command", "checker code", "review risks", "threat model", "port or copy", "is also a review risk", "the other direction", "Put the checker conflicts first", "under 600 words"}
	advisorSteps   = []string{templateName, "go test -list", "review risk", "threat model", "800 lines", "under 500 words"}
)

func harnessBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), binaryName)
	build := exec.Command("go", "build", "-o", bin, "./cmd/harness")
	build.Dir = repository.Root(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func subcommands(t *testing.T, bin string, parent ...string) []string {
	t.Helper()
	out, err := exec.Command(bin, append(parent, helpFlag)...).Output()
	if err != nil {
		t.Fatalf("%s %v: %v", binaryName, parent, err)
	}
	var names []string
	for _, m := range helpCommand.FindAllStringSubmatch(string(out), -1) {
		names = append(names, m[1])
	}
	return names
}

func checkBrief(t *testing.T, path string, steps []string) {
	t.Helper()
	raw := repository.Read(t, path)
	for _, want := range steps {
		if !strings.Contains(raw, want) {
			t.Errorf("%s does not hold %q", path, want)
		}
	}
	repository.Read(t, templateRoot+templateName)
	bin := harnessBinary(t)
	for _, m := range harnessCommand.FindAllStringSubmatch(raw, -1) {
		words := strings.Fields(m[1])
		if !slices.Contains(subcommands(t, bin), words[0]) {
			t.Errorf("%s names the unknown command %q", path, m[0])
			continue
		}
		if len(words) > 1 && !slices.Contains(subcommands(t, bin, words[0]), words[1]) {
			t.Errorf("%s names the unknown command %q", path, m[0])
		}
	}
}

func TestExplorerBrief(t *testing.T) { checkBrief(t, explorerFile, explorerSteps) }

func TestAdvisorBrief(t *testing.T) { checkBrief(t, advisorFile, advisorSteps) }

func TestAgentFiles(t *testing.T) {
	want := repository.FrontmatterKeys(t, reviewerAgent)
	for _, path := range []string{advisorFile, workerFile, verifierAgent} {
		if got := repository.FrontmatterKeys(t, path); !slices.Equal(got, want) {
			t.Errorf("%s frontmatter keys = %v, want the keys of %s: %v", path, got, reviewerAgent, want)
		}
	}
}

func TestAgentReportWordLimit(t *testing.T) {
	agents, err := repository.Glob(t, agentDocs)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range agents {
		if !reportLimit.MatchString(repository.Read(t, path)) {
			t.Errorf("%s gives no word limit for its report", path)
		}
	}
}

func TestAgentBriefsPinReviewRound(t *testing.T) {
	for path, wants := range map[string][]string{
		reviewerAgent: {findingLine, filesReadLine, reReviewTitle, standardsRule, reviewAnswer},
		verifierAgent: {verifierLine},
		workerFile:    {workerFixLine},
	} {
		raw := repository.Read(t, path)
		for _, want := range wants {
			if !strings.Contains(raw, want) {
				t.Errorf("%s does not hold %q", path, want)
			}
		}
	}
}

func TestVerifierBriefTakesRound(t *testing.T) {
	for path, wants := range map[string][]string{
		verifierAgent: {"all findings of a round", "one line for each finding", "no verifier line counts as real"},
		reviewerAgent: {"all fixups of a round", "git show <sha>", "whether its fixup fixes it"},
	} {
		raw := repository.Read(t, path)
		for _, want := range wants {
			if !strings.Contains(raw, want) {
				t.Errorf("%s does not hold %q", path, want)
			}
		}
	}
}
