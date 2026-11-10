package agents_test

import (
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	replayDir       = "agents/testdata/replay/"
	readmeFocus     = replayDir + "readme-focus.md"
	dddDepth        = replayDir + "ddd-depth.md"
	guardPrecision  = replayDir + "r2-guard-precision.md"
	explorerHeading = "## Explorer report"
	advisorHeading  = "## Advisor report"
	untilReportEnd  = ""
)

type replayCheck struct {
	fixture string
	heading string
	from    string
	to      string
	wants   []string
}

var replayChecks = []replayCheck{
	{readmeFocus, advisorHeading, "", untilReportEnd, []string{"release not found", "Decide:"}},
	{dddDepth, explorerHeading, "CHECKER CONFLICTS", "FILES TOUCHED", []string{"domain port", "clock"}},
	{dddDepth, explorerHeading, "OPEN DECISIONS", untilReportEnd, []string{"(C2, C3)"}},
	{guardPrecision, explorerHeading, "Review risks", "Not done", []string{"cp -t"}},
	{guardPrecision, explorerHeading, "Threat model", "Review risks", []string{"Fail closed:"}},
}

func reportOf(doc, heading string) string {
	_, rest, _ := strings.Cut(doc, heading+"\n")
	for _, other := range []string{explorerHeading, advisorHeading} {
		if other != heading {
			rest, _, _ = strings.Cut(rest, "\n"+other)
		}
	}
	return rest
}

func section(report, from, to string) (string, bool) {
	lines := strings.Split(report, "\n")
	start := 0
	if from != "" {
		start = -1
		for i, l := range lines {
			if strings.Contains(l, from) {
				start = i
				break
			}
		}
		if start < 0 {
			return "", false
		}
	}
	end := len(lines)
	if to != "" {
		end = -1
		for i := start + 1; i < len(lines); i++ {
			if strings.Contains(lines[i], to) {
				end = i
				break
			}
		}
		if end < 0 {
			return "", false
		}
	}
	return strings.Join(lines[start:end], "\n"), true
}

func TestReplay(t *testing.T) {
	for _, c := range replayChecks {
		report := reportOf(repository.Read(t, c.fixture), c.heading)
		got, ok := section(report, c.from, c.to)
		if !ok {
			t.Errorf("%s %s: section %q to %q not found", c.fixture, c.heading, c.from, c.to)
			continue
		}
		for _, want := range c.wants {
			if !strings.Contains(got, want) {
				t.Errorf("%s %s: section %q to %q lacks %q", c.fixture, c.heading, c.from, c.to, want)
			}
		}
	}
}
