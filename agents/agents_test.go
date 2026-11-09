package agents_test

import (
	"regexp"
	"slices"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	advisorFile   = "agents/advisor.md"
	workerFile    = "agents/worker.md"
	agentDocs     = "agents/*.md"
	reviewerAgent = "agents/reviewer.md"
)

var (
	reportLimit = regexp.MustCompile(`\b(?:under|at most) \d+ words\b`)
)

func TestAgentFiles(t *testing.T) {
	want := repository.FrontmatterKeys(t, reviewerAgent)
	for _, path := range []string{advisorFile, workerFile} {
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
