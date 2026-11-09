package report_test

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	reportdomain "github.com/90p-labs/harness/internal/report/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	issueTemplateFile = "skills/report/references/issue-template.md"
	allSections       = "Summary, Reproduction, Reproduction > Commands, Reproduction > Config, Reproduction > Expected, " +
		"Reproduction > Actual, Traces, Traces > Host, Traces > Output, Fix suggestion, Workaround"
)

var (
	templatePlaceholder = regexp.MustCompile(`\[fill: [^\]]+\]`)
)

func TestReportTemplate(t *testing.T) {
	raw, err := os.ReadFile(repository.Path(t, issueTemplateFile))
	if err != nil {
		t.Fatal(err)
	}
	template := string(raw)

	err = reportdomain.Draft{Title: "t", Body: template}.Check()
	if !errors.Is(err, reportdomain.ErrDraftIncomplete) || !strings.HasSuffix(err.Error(), ": "+allSections) {
		t.Fatalf("the unfilled template: Check() = %v, want every section missing", err)
	}

	filled := templatePlaceholder.ReplaceAllString(template, "filled")
	if err := (reportdomain.Draft{Title: "t", Body: filled}).Check(); err != nil {
		t.Fatalf("the filled template: Check() = %v, want nil", err)
	}
}
