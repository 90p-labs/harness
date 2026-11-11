package docs_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	verifyReadme       = "README.md"
	verifyDesign       = "DESIGN.md"
	verifyConfigPage   = "docs/configuration.md"
	verifyRunsPage     = "docs/runs.md"
	verifySetupPage    = "docs/setup.md"
	verifyExamplesDoc  = "examples/README.md"
	verifyGoExample    = "examples/go/README.md"
	verifyNextExample  = "examples/nextjs/README.md"
	verifyPyExample    = "examples/python/README.md"
	verifySymfonyDoc   = "examples/symfony/README.md"
	oldPassGatePattern = `(?m)^\s*(PASS|CACHED) [a-z0-9-]+ [0-9.]+(ms|s|m)`
	oldCommitPattern   = `(?m)^\s*(PASS|FAIL) [0-9a-f]{12} [a-z]+$`
	basePattern        = `^\s*base \S+ [0-9a-f]{12}$`
	passCommitsPrefix  = "PASS commits="
)

var (
	verifyDocs = []string{
		verifyReadme, verifyDesign, verifyConfigPage, verifyRunsPage, verifySetupPage,
		verifyExamplesDoc, verifyGoExample, verifyNextExample, verifyPyExample, verifySymfonyDoc,
	}
	oldPassGateLine = regexp.MustCompile(oldPassGatePattern)
	oldCommitLine   = regexp.MustCompile(oldCommitPattern)
	baseLine        = regexp.MustCompile(basePattern)
)

func TestDocsShowTheVerifyCommitsBase(t *testing.T) {
	for _, doc := range verifyDocs {
		lines := strings.Split(repository.Read(t, doc), "\n")
		for i, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), passCommitsPrefix) {
				continue
			}
			if i == 0 || !baseLine.MatchString(lines[i-1]) {
				t.Errorf("%s line %d: %q has no base line before it", doc, i+1, line)
			}
		}
	}
	design := repository.Read(t, verifyDesign)
	for _, want := range []string{"origin/<base>", "so a stale local branch lists no other pull request commits"} {
		if !strings.Contains(design, want) {
			t.Errorf("%s does not name %q", verifyDesign, want)
		}
	}
}

func TestDocsShowShortVerifyOutput(t *testing.T) {
	for _, doc := range verifyDocs {
		text := repository.Read(t, doc)
		if line := oldPassGateLine.FindString(text); line != "" {
			t.Errorf("%s shows the old pass gate line %q", doc, line)
		}
		if line := oldCommitLine.FindString(text); line != "" {
			t.Errorf("%s shows the old commit line %q", doc, line)
		}
	}
}
