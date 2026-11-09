package domain_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/report/domain"
)

const completeBody = `## Summary

harness hook exits 2 on a Stop hook with a bad payload, and Claude Code blocks the stop forever.

## Reproduction

### Commands

` + "```sh" + `
git init demo && cd demo
harness host install --host claude
echo '{' | harness hook --host claude stop
` + "```" + `

### Config

` + "```yaml" + `
run:
  approval: true
` + "```" + `

### Expected

The hook prints the allow output and exits 0.

### Actual

The hook prints "error: unexpected EOF" and exits 2.

## Traces

### Host

Claude Code 2.1.3

### Output

` + "````text" + `
error: unexpected EOF
` + "```" + `
## Summary

a heading inside a nested fence
` + "```" + `
` + "````" + `

## Fix suggestion

cmd/harness/main.go, run: map a hook decode error to exit 1.

## Workaround

Remove the Stop hook from .claude/settings.json.
`

func TestCheckDraft(t *testing.T) {
	cases := []struct {
		name        string
		body        string
		wantMissing []string
	}{
		{name: "a complete draft passes", body: completeBody},
		{
			name:        "an absent section is missing",
			body:        strings.Replace(completeBody, "## Workaround\n\nRemove the Stop hook from .claude/settings.json.\n", "", 1),
			wantMissing: []string{"Workaround"},
		},
		{
			name:        "an empty sub-section is missing",
			body:        strings.Replace(completeBody, "The hook prints the allow output and exits 0.\n", "", 1),
			wantMissing: []string{"Reproduction > Expected"},
		},
		{
			name:        "a section with its placeholder is missing",
			body:        strings.Replace(completeBody, "Claude Code 2.1.3", "[fill: the host and its version]", 1),
			wantMissing: []string{"Traces > Host"},
		},
		{
			name:        "a parent with only unfilled sub-sections is missing",
			body:        "## Traces\n\n### Host\n\n[fill: the host]\n\n### Output\n\n[fill: the output]\n",
			wantMissing: []string{"Summary", "Reproduction", "Reproduction > Commands", "Reproduction > Config", "Reproduction > Expected", "Reproduction > Actual", "Traces", "Traces > Host", "Traces > Output", "Fix suggestion", "Workaround"},
		},
		{
			name:        "a heading inside a fence does not count",
			body:        strings.Replace(completeBody, "## Summary\n\nharness hook exits 2 on a Stop hook with a bad payload, and Claude Code blocks the stop forever.\n\n", "", 1),
			wantMissing: []string{"Summary"},
		},
		{
			name:        "a fence with no content is missing",
			body:        strings.Replace(completeBody, "git init demo && cd demo\nharness host install --host claude\necho '{' | harness hook --host claude stop\n", "", 1),
			wantMissing: []string{"Reproduction > Commands"},
		},
		{
			name: "an inline code span does not open a fence",
			body: strings.Replace(completeBody, "and Claude Code blocks the stop forever.", "and Claude Code blocks the stop forever.\n```harness hook --host claude stop``` exits 2.", 1),
		},
		{
			name: "every missing section is named in template order",
			body: "## Traces\n\n### Output\n\nok\n",
			wantMissing: []string{
				"Summary", "Reproduction", "Reproduction > Commands", "Reproduction > Config",
				"Reproduction > Expected", "Reproduction > Actual", "Traces > Host", "Fix suggestion", "Workaround",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.Draft{Title: "hook exits 2", Body: tc.body}.Check()
			if len(tc.wantMissing) == 0 {
				if err != nil {
					t.Fatalf("Check() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, domain.ErrDraftIncomplete) {
				t.Fatalf("Check() = %v, want ErrDraftIncomplete", err)
			}
			if want := ": " + strings.Join(tc.wantMissing, ", "); !strings.HasSuffix(err.Error(), want) {
				t.Fatalf("Check() = %q, want the suffix %q", err, want)
			}
		})
	}
}

func TestDraftIssue(t *testing.T) {
	draft := domain.Draft{
		Title: "hook fails for jorge@example.com",
		Body:  strings.Replace(completeBody, "Claude Code 2.1.3", "Claude Code 2.1.3 in /Users/jorge/work/demo", 1),
	}
	env := domain.Environment{Version: "v0.4.0", Platform: "darwin/arm64", Run: domain.RunNone, Home: "/Users/jorge"}

	issue := draft.Issue(env)

	if issue.Title != "hook fails for <redacted>" {
		t.Fatalf("title = %q, want the email redacted", issue.Title)
	}
	wantTraces := "## Traces\n\n### Environment\n\n- harness: v0.4.0\n- os: darwin/arm64\n- run: none\n\n### Host\n\nClaude Code 2.1.3 in ~/work/demo\n"
	if !strings.Contains(issue.Body, wantTraces) {
		t.Fatalf("body misses the stamped and redacted traces %q:\n%s", wantTraces, issue.Body)
	}
	if strings.Count(issue.Body, "### Environment") != 1 {
		t.Fatalf("body has more than one Environment block:\n%s", issue.Body)
	}
	if err := issue.Check(); err != nil {
		t.Fatalf("the issue fails its own check: %v", err)
	}
}

func TestRunLine(t *testing.T) {
	if got := domain.RunLine("20261108-fix-hook", "working"); got != "20261108-fix-hook, phase working" {
		t.Fatalf("RunLine = %q", got)
	}
}
