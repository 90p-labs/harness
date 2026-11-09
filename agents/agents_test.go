package agents_test

import (
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
	reviewAnswer  = "Answer `closed` or `open` for the finding."
	workerFixLine = "For a fix, the lead gives finding lines in place of a task. Your scope is the file of each finding and its test."
)

var (
	reportLimit = regexp.MustCompile(`\b(?:under|at most) \d+ words\b`)
)

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
