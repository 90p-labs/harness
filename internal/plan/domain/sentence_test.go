package domain_test

import (
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/plan/domain"
)

func words(n int) string {
	return strings.TrimSpace(strings.Repeat("word ", n))
}

func TestSentenceLength(t *testing.T) {
	valid := readValid(t)
	const goalLine = "The service answers a ping with the word pong. The query records one span for each call."
	const goalLineNumber = 5
	const tableRow = "| 3 | The module builds and passes all tests. |"

	tests := []struct {
		name     string
		edit     edit
		wantLine int
	}{
		{"25 words", edit{goalLine, words(25) + "."}, 0},
		{"26 words", edit{goalLine, words(26) + "."}, goalLineNumber},
		{"backticked span is one word", edit{goalLine, words(24) + " `a b c d e`."}, 0},
		{"two short sentences", edit{goalLine, words(20) + ". " + words(20) + "."}, 0},
		{"question and bang end a sentence", edit{goalLine, words(20) + "? " + words(20) + "!"}, 0},
		{"colon and semicolon do not end a sentence", edit{goalLine, words(13) + ": " + words(13) + "."}, goalLineNumber},
		{"dot inside a name does not end a sentence", edit{goalLine, words(25) + " plan.md more."}, goalLineNumber},
		{"no terminator", edit{goalLine, words(26)}, goalLineNumber},
		{"code fence is skipped", edit{goalLine, "```text\n" + words(40) + ".\n```"}, 0},
		{"indented backticks open no fence", edit{goalLine, "  ```\n" + words(26) + "."}, goalLineNumber + 1},
		{"table cell counts alone", edit{tableRow, "| 3 | " + words(26) + ". |"}, 19},
		{"table cells do not add up", edit{tableRow, "| 3 | " + words(20) + ". | " + words(20) + " |"}, 0},
		{"list marker is not a word", edit{"- `skills/ddd-conventions/SKILL.md`", "- " + words(25)}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := byRule(checkText(mutate(t, valid, tc.edit), domain.CheckInput{}), domain.RuleSentenceLength)
			if tc.wantLine == 0 {
				if len(got) != 0 {
					t.Fatalf("problems = %v, want none", got)
				}
				return
			}
			if len(got) != 1 || got[0].Line != tc.wantLine {
				t.Fatalf("problems = %v, want one at line %d", got, tc.wantLine)
			}
		})
	}
}
