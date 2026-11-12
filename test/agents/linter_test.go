package agents_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	linterAgent  = "agents/linter.md"
	proofCommand = "harness lint canaries --proof"
	proofFolder  = "lint/proof/"
)

var (
	committedProofPaths = []string{"lint/rules.yaml", "lint/canaries"}
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

func TestLintSetupAgent(t *testing.T) {
	requireProof(t, linterAgent, repository.Read(t, linterAgent))
	got, want := repository.FrontmatterKeys(t, linterAgent), repository.FrontmatterKeys(t, reviewerAgent)

	if !slices.Equal(got, want) {
		t.Errorf("%s frontmatter keys = %v, want the keys of %s: %v", linterAgent, got, reviewerAgent, want)
	}
}
