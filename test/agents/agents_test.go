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
	advisorFile      = "agents/advisor.md"
	workerFile       = "agents/worker.md"
	agentDocs        = "agents/*.md"
	reviewerAgent    = "agents/reviewer.md"
	verifierAgent    = "agents/verifier.md"
	findingLine      = "`<id> | <severity> | <file>:<line> | <defect and failure scenario> | <observed or inferred> | <proof>`"
	verifierLine     = "`<id> | real or not real | <severity> | <reason>`"
	filesReadLine    = "Files read:"
	reReviewTitle    = "## Re-review"
	standardsRule    = "does not report a rule that a gate checks"
	reviewAnswer     = "Answer `closed` or `open`, to say whether its fixup fixes it."
	explorerFile     = "agents/explorer.md"
	templateName     = "references/plan-template.md"
	templateRoot     = "skills/harness/"
	binaryName       = "harness"
	helpFlag         = "--help"
	workerFixLine    = "For a fix, the lead gives finding lines in place of a task. Your scope is the file of each finding and its test."
	workerIntro      = "You implement one task of a plan, one fix or one self-check. The lead gives you the path of the harness binary, the path of `plan.md` and one input."
	workerCheck      = "For a self-check, the lead gives the self-check brief of the harness skill."
	workerTask       = "For a task, the lead gives the number of the task."
	workerRead       = "For a fix, read the file of each finding, its test and the values that `plan.md` sets for them."
	baselineTrigger  = "modifies, replaces or removes existing behaviour"
	baselineSkip     = "`baseline: not needed: <one-line reason>`"
	classIntended    = "`intended`"
	classBug         = "`bug`"
	classAccidental  = "`accidental`"
	classAmbiguous   = "`ambiguous`"
	tautologicalLine = "`tautological`: the test computes the expected value the way the code computes it, so the test cannot fail."
	coupledLine      = "`implementation-coupled`: the test mocks an internal part, or checks the result through a side channel in place of the seam."
	horizontalLine   = "`horizontal slicing`: all the tests come first and all the code comes after, in place of one behaviour at a time."
	mockRuleLine     = "Mock only at a system boundary: a network service, the clock, randomness, and sometimes a database or the file system."
	seamWord         = "seam"
	workerPlan       = "Keep each value, name and limit that `plan.md` sets. When your change needs another one, skip that change, name the conflict in the report and end with `ISSUES`. In a self-check, write the conflict as `open` in `self-check.md`, with `plan conflict:` in the evidence."
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

var explorerBaseline = []string{
	"## Evidence baseline",
	baselineTrigger,
	baselineSkip,
	"This holds unless an ADR, `CONTEXT.md` entry or test fully pins that behaviour.",
	"behaviour that is already fully pinned",
	"When only part of the behaviour is pinned, the baseline covers the part that nothing pins.",
	classIntended, classBug, classAccidental, classAmbiguous,
	"`#`, `Behaviour`, `Code`, `Test`, `Values`, `Anomaly` and `Class`",
	"When the code and a test disagree, the test states the intent.",
	"A behaviour that only a doc describes is an aspiration.",
	"one table for each independent area",
	"The tables may add 600 words more in total.",
	"An `ambiguous` baseline item stays in its table and is not repeated as an open decision.",
	"touched files, evidence baseline or its `not needed` line, review risks",
}

var explorerSources = []string{
	"- A contract states what the system promises",
	"- The code states what the system does.",
	"- A test is decisive.",
	"- A doc helps to find the code.",
}

func TestExplorerBaseline(t *testing.T) {
	raw := repository.Read(t, explorerFile)
	for _, want := range explorerBaseline {
		if !strings.Contains(raw, want) {
			t.Errorf("%s does not hold %q", explorerFile, want)
		}
	}
	_, order, _ := strings.Cut(raw, "## Evidence baseline")
	last := -1
	for _, bullet := range explorerSources {
		at := strings.Index(order, bullet)
		if at < 0 {
			t.Errorf("%s source bullet %q is missing", explorerFile, bullet)
			continue
		}
		if at < last {
			t.Errorf("%s source bullet %q is out of order", explorerFile, bullet)
		}
		last = at
	}
	if !regexp.MustCompile(`(?m)^3\. .*evidence baseline`).MatchString(raw) {
		t.Errorf("%s step 3 does not decide the evidence baseline", explorerFile)
	}
	desc := regexp.MustCompile(`(?m)^description: .*`).FindString(raw)
	if !strings.Contains(desc, "evidence baseline") {
		t.Errorf("%s description does not name the evidence baseline", explorerFile)
	}
}

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
		workerFile:    {workerIntro, workerTask, workerFixLine, workerCheck, workerRead, workerPlan},
	} {
		raw := repository.Read(t, path)
		for _, want := range wants {
			if !strings.Contains(raw, want) {
				t.Errorf("%s does not hold %q", path, want)
			}
		}
	}
}

func TestAgentBriefsTestAntiPatterns(t *testing.T) {
	lines := []string{tautologicalLine, coupledLine, horizontalLine, mockRuleLine, seamWord}
	for _, path := range []string{workerFile, reviewerAgent} {
		raw := repository.Read(t, path)
		for _, want := range lines {
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
