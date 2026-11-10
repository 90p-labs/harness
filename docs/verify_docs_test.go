package docs_test

import (
	"regexp"
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
)

var (
	verifyDocs = []string{
		verifyReadme, verifyDesign, verifyConfigPage, verifyRunsPage, verifySetupPage,
		verifyExamplesDoc, verifyGoExample, verifyNextExample, verifyPyExample, verifySymfonyDoc,
	}
	oldPassGateLine = regexp.MustCompile(oldPassGatePattern)
	oldCommitLine   = regexp.MustCompile(oldCommitPattern)
)

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
