package domain_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const emoji = "\U0001F600"

func words(n int) string {
	return strings.TrimSpace(strings.Repeat("word ", n))
}

func sentences(n int) string {
	return strings.TrimSpace(strings.Repeat("One short sentence. ", n))
}

func lengthMessage(n int) string {
	return fmt.Sprintf("sentence has %d words, the limit is %d", n, domain.MaxProseSentenceWords)
}

func paragraphMessage(n int) string {
	return fmt.Sprintf("paragraph has %d sentences, the limit is %d", n, domain.MaxProseParagraphSentences)
}

func everyLine(int) bool { return true }

func TestProseSentences(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []domain.ProseProblem
	}{
		{"25 words pass", []string{words(25) + "."}, nil},
		{"26 words fail", []string{words(26) + "."}, []domain.ProseProblem{{Line: 1, Rule: domain.ProseSentenceLength, Message: lengthMessage(26)}}},
		{"em dash", []string{"a " + repository.EmDash + " b"}, []domain.ProseProblem{{Line: 1, Rule: domain.ProseEmDash, Message: "the line has an em dash"}}},
		{"em dash in a fence", []string{"```", "a " + repository.EmDash + " b", "```"}, []domain.ProseProblem{{Line: 2, Rule: domain.ProseEmDash, Message: "the line has an em dash"}}},
		{"long text in a fence", []string{"```", words(40), "```"}, nil},
		{"code span is one word", []string{"`" + words(40) + "` " + words(24) + "."}, nil},
		{"list mark is stripped", []string{"- " + words(25) + "."}, nil},
		{"heading mark is stripped", []string{"## " + words(25) + "."}, nil},
		{"table cells count alone", []string{"| " + words(20) + " | " + words(20) + " |", "| --- | --- |"}, nil},
		{"table cell over the limit", []string{"| a | " + words(26) + " |"}, []domain.ProseProblem{{Line: 1, Rule: domain.ProseSentenceLength, Message: lengthMessage(26)}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.CheckProseSentences(tc.lines)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProseParagraphs(t *testing.T) {
	long := sentences(7)
	tests := []struct {
		name  string
		lines []string
		want  []domain.ProseProblem
	}{
		{"6 sentences pass", []string{sentences(6)}, nil},
		{"7 sentences fail at the first line", []string{"", sentences(7)}, []domain.ProseProblem{{Line: 2, Rule: domain.ProseParagraphLength, Message: paragraphMessage(7)}}},
		{"lines join into one paragraph", []string{sentences(4), sentences(3)}, []domain.ProseProblem{{Line: 1, Rule: domain.ProseParagraphLength, Message: paragraphMessage(7)}}},
		{"blank line splits", []string{sentences(4), "", sentences(3)}, nil},
		{"heading splits", []string{sentences(4), "## Title", sentences(3)}, nil},
		{"list item splits", []string{sentences(4), "- item", sentences(3)}, nil},
		{"table row splits", []string{sentences(4), "| a | b |", sentences(3)}, nil},
		{"fence splits and its content is skipped", []string{sentences(4), "```", sentences(9), "```", sentences(3)}, nil},
		{"frontmatter is skipped", []string{"---", sentences(9), "---", sentences(3)}, nil},
		{"list item is a paragraph", []string{"- " + long}, []domain.ProseProblem{{Line: 1, Rule: domain.ProseParagraphLength, Message: paragraphMessage(7)}}},
		{"list items do not join", []string{"- " + sentences(4), "- " + sentences(3)}, nil},
		{"code span full stop is not a sentence end", []string{sentences(5) + " One `a. b. c.` sentence."}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.CheckProse(tc.lines, everyLine)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProseIndentedFence(t *testing.T) {
	lines := []string{"- item", "  ```text", "  " + words(30) + ".", "  ```", words(30) + "."}
	want := []domain.ProseProblem{{Line: 5, Rule: domain.ProseSentenceLength, Message: lengthMessage(30)}}
	if got := domain.CheckProse(lines, everyLine); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestProseChangedLinesOnly(t *testing.T) {
	only := func(n int) func(int) bool { return func(line int) bool { return line == n } }
	lengthAt := func(line int) []domain.ProseProblem {
		return []domain.ProseProblem{{Line: line, Rule: domain.ProseSentenceLength, Message: lengthMessage(30)}}
	}
	tests := []struct {
		name    string
		lines   []string
		changed func(int) bool
		want    []domain.ProseProblem
	}{
		{"old long sentence is ignored", []string{words(30) + ".", "short."}, only(2), nil},
		{"changed long sentence is reported", []string{words(30) + ".", "short."}, only(1), lengthAt(1)},
		{"last changed line reports the paragraph at its first line", []string{sentences(4), sentences(3)}, only(2), []domain.ProseProblem{{Line: 1, Rule: domain.ProseParagraphLength, Message: paragraphMessage(7)}}},
		{"untouched paragraph is ignored", []string{sentences(4), sentences(3), "", "new."}, only(4), nil},
		{"emoji on a changed line", []string{"old " + emoji, "new " + emoji}, only(2), []domain.ProseProblem{{Line: 2, Rule: domain.ProseEmoji, Message: "the line has an emoji"}}},
		{"emoji in a fence is reported", []string{"```", emoji, "```"}, only(2), []domain.ProseProblem{{Line: 2, Rule: domain.ProseEmoji, Message: "the line has an emoji"}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.CheckProse(tc.lines, tc.changed)
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
