package harness

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/report/domain"
)

const (
	issueTemplateFile = "skills/report/references/issue-template.md"
	allSections       = "Summary, Reproduction, Reproduction > Commands, Reproduction > Config, Reproduction > Expected, " +
		"Reproduction > Actual, Traces, Traces > Host, Traces > Output, Fix suggestion, Workaround"
)

var templatePlaceholder = regexp.MustCompile(`\[fill: [^\]]+\]`)

func TestReportTemplate(t *testing.T) {
	raw, err := os.ReadFile(issueTemplateFile)
	if err != nil {
		t.Fatal(err)
	}
	template := string(raw)

	err = domain.Draft{Title: "t", Body: template}.Check()
	if !errors.Is(err, domain.ErrDraftIncomplete) || !strings.HasSuffix(err.Error(), ": "+allSections) {
		t.Fatalf("the unfilled template: Check() = %v, want every section missing", err)
	}

	filled := templatePlaceholder.ReplaceAllString(template, "filled")
	if err := (domain.Draft{Title: "t", Body: filled}).Check(); err != nil {
		t.Fatalf("the filled template: Check() = %v, want nil", err)
	}
}
