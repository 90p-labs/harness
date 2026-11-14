package docs_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	followUpsDoc      = "docs/follow-ups.md"
	followUpsOpen     = "\n## Open\n"
	followUpsNext     = "\n## "
	followUpsEntry    = "\n### "
	categoryPrefix    = "- Category: "
	citedByPrefix     = "- Cited by:"
	promptPrefix      = "- Prompt:"
	promptCommand     = "harness run start"
	promptRemoval     = "Remove the entry of this cause from docs/follow-ups.md"
	backtick          = "`"
	citedByBackticks  = 4
	promptBackticks   = 2
	problemNoOpen     = "no Open heading"
	problemEntryAbove = "entry heading above Open"
	problemNoCategory = "no category"
	problemBadCat     = "unknown category"
	problemNoCited    = "no Cited by line"
	problemFewSpans   = "Cited by line with fewer than two code spans"
	problemNoPrompt   = "no Prompt line"
	problemNoCommand  = "Prompt line with no harness run start"
	problemNoSpan     = "Prompt line with no code span"
	problemNoRemoval  = "Prompt line with no removal request"
	problemTwoPrompts = "more than one Prompt line"
	problemStaleText  = "text between Open and the first entry"
)

const conventionCategory = "Convention"

var followUpCategories = append(slices.Clone(lessonCategories), conventionCategory)

const (
	goodPromptText = "harness run start \"Add the check. " + promptRemoval + ".\""
	goodPromptLine = promptPrefix + " " + backtick + goodPromptText + backtick
	goodFollowUp   = `
### The gate skips the docs folder

- Category: Automated checks
- Cited by: ` + "`20261114-100000-a`, `rework-1`" + `
- Cited by: ` + "`20261115-100000-b`, `docs/a.md:3`" + `
` + goodPromptLine + `
`
)

// followUpProblems returns one problem for each mistake in the entries under the section Open.
func followUpProblems(doc string) []string {
	before, section, found := strings.Cut("\n"+doc+"\n", followUpsOpen)
	if !found {
		return []string{problemNoOpen}
	}
	var problems []string
	if strings.Contains(before, followUpsEntry) {
		problems = append(problems, problemEntryAbove)
	}
	section, _, _ = strings.Cut("\n"+section, followUpsNext)
	entries := strings.Split(section, followUpsEntry)
	if len(entries) > 1 && strings.TrimSpace(entries[0]) != "" {
		problems = append(problems, problemStaleText)
	}
	for _, entry := range entries[1:] {
		heading, body, _ := strings.Cut(entry, "\n")
		var category, prompt string
		var cited []string
		var prompts int
		for _, line := range strings.Split(body, "\n") {
			switch {
			case strings.HasPrefix(line, categoryPrefix):
				category = strings.TrimPrefix(line, categoryPrefix)
			case strings.HasPrefix(line, citedByPrefix):
				cited = append(cited, line)
			case strings.HasPrefix(line, promptPrefix):
				prompt = line
				prompts++
			}
		}
		add := func(problem string) { problems = append(problems, heading+": "+problem) }
		switch {
		case category == "":
			add(problemNoCategory)
		case !slices.Contains(followUpCategories, category):
			add(problemBadCat)
		}
		if len(cited) == 0 {
			add(problemNoCited)
		}
		for _, line := range cited {
			if strings.Count(line, backtick) < citedByBackticks {
				add(problemFewSpans)
			}
		}
		if prompts > 1 {
			add(problemTwoPrompts)
		}
		switch {
		case prompt == "":
			add(problemNoPrompt)
		case !strings.Contains(prompt, promptCommand):
			add(problemNoCommand)
		case strings.Count(prompt, backtick) < promptBackticks:
			add(problemNoSpan)
		case !strings.Contains(prompt, promptRemoval):
			add(problemNoRemoval)
		}
	}
	return problems
}

func TestFollowUpsFileKeepsTheEntryForm(t *testing.T) {
	for _, problem := range followUpProblems(repository.Read(t, followUpsDoc)) {
		t.Errorf("%s: %s", followUpsDoc, problem)
	}
}

func TestFollowUpEntryProblems(t *testing.T) {
	const head = "# Follow-ups\n\n## Open\n"
	cases := []struct {
		name  string
		entry string
		want  string
	}{
		{"good entry", goodFollowUp, ""},
		{"no entry", "\nNo open follow-up.\n", ""},
		{"no category", strings.Replace(goodFollowUp, "- Category: Automated checks\n", "", 1), problemNoCategory},
		{"unknown category", strings.Replace(goodFollowUp, "Automated checks", "Speed", 1), problemBadCat},
		{"convention category", strings.Replace(goodFollowUp, "Automated checks", conventionCategory, 1), ""},
		{"no cited by", strings.Join(removeLines(goodFollowUp, citedByPrefix), "\n"), problemNoCited},
		{"one code span", strings.Replace(goodFollowUp, ", `rework-1`", "", 1), problemFewSpans},
		{"no prompt", strings.Join(removeLines(goodFollowUp, promptPrefix), "\n"), problemNoPrompt},
		{"no command", strings.Replace(goodFollowUp, promptCommand, "run", 1), problemNoCommand},
		{"no prompt code span", strings.Replace(goodFollowUp, goodPromptLine, promptPrefix+" "+goodPromptText, 1), problemNoSpan},
		{"no removal request", strings.Replace(goodFollowUp, promptRemoval, "Keep it", 1), problemNoRemoval},
		{"two prompt lines", goodFollowUp + goodPromptLine + "\n", problemTwoPrompts},
		{"two entries", goodFollowUp + strings.Replace(goodFollowUp, "The gate skips the docs folder", "Another cause", 1), ""},
		{"text above the first entry", "\nNo follow-up is open.\n" + goodFollowUp, problemStaleText},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkProblems(t, followUpProblems(head+tc.entry), tc.want)
		})
	}
}

func TestFollowUpLayoutProblems(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want string
	}{
		{"open heading", "# Follow-ups\n\n## Open\n" + goodFollowUp, ""},
		{"open heading at the end", "# Follow-ups\n\n## Open", ""},
		{"no open heading", "#Follow-ups\n\nNo follow-up is open.\n", problemNoOpen},
		{"open as level three", "# Follow-ups\n\n### Open\n" + goodFollowUp, problemNoOpen},
		{"entry form above open", "# Follow-ups\n\n### <the cause in one line>\n\n## Open\n\nNo follow-up is open.\n", problemEntryAbove},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkProblems(t, followUpProblems(tc.doc), tc.want)
		})
	}
}

func checkProblems(t *testing.T, problems []string, want string) {
	t.Helper()
	if want == "" {
		if len(problems) != 0 {
			t.Fatalf("unexpected problems: %v", problems)
		}
		return
	}
	if len(problems) != 1 || !strings.HasSuffix(problems[0], want) {
		t.Fatalf("problems = %v, want one that ends with %q", problems, want)
	}
}

func removeLines(text, prefix string) []string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, prefix) {
			kept = append(kept, line)
		}
	}
	return kept
}
