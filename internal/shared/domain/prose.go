package domain

import (
	"fmt"
	"regexp"
	"strings"
)

type ProseRule string

const (
	ProseSentenceLength  ProseRule = "sentence-length"
	ProseParagraphLength ProseRule = "paragraph-length"
	ProseEmDash          ProseRule = "em-dash"
	ProseEmoji           ProseRule = "emoji"
)

const (
	MaxProseSentenceWords      = 25
	MaxProseParagraphSentences = 6

	proseFenceMark   = "```"
	proseFrontMatter = "---"
	proseTableMark   = "|"
	proseHeadingMark = "#"
	proseTrailing    = " \t\r"
	proseIndent      = " \t"
	proseCellEscape  = "\x00"
	proseCodeWord    = "code"
	proseEmDash      = string(rune(0x2014))
)

type ProseProblem struct {
	Line    int
	Rule    ProseRule
	Message string
}

var (
	proseCodeSpan    = regexp.MustCompile("`[^`]*`")
	proseSentenceEnd = regexp.MustCompile(`[.!?]+(?:\s+|$)`)
	proseListMarker  = regexp.MustCompile(`^\s*(?:[-*]|\d+\.)\s+`)
	proseHeading     = regexp.MustCompile(`^#+\s+`)
	proseSeparator   = regexp.MustCompile(`^:?-+:?$`)
	proseIndented    = regexp.MustCompile(`^\s+\S`)
)

func IsFenceOpen(line string) bool {
	return strings.HasPrefix(line, proseFenceMark)
}

func IsFenceClose(line string) bool {
	return line == proseFenceMark
}

func MaskCodeSpans(text string) string {
	return proseCodeSpan.ReplaceAllString(text, proseCodeWord)
}

func fenceText(line string, indented bool) string {
	if indented {
		return strings.TrimLeft(line, proseIndent)
	}
	return line
}

func TableCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimSuffix(strings.TrimPrefix(line, proseTableMark), proseTableMark)
	parts := strings.Split(strings.ReplaceAll(line, `\|`, proseCellEscape), proseTableMark)
	for i, part := range parts {
		parts[i] = strings.TrimSpace(strings.ReplaceAll(part, proseCellEscape, proseTableMark))
	}
	return parts
}

func IsTableSeparator(cells []string) bool {
	for _, cell := range cells {
		if !proseSeparator.MatchString(cell) {
			return false
		}
	}
	return true
}

func IsEmoji(r rune) bool {
	return r >= 0x1F300 && r <= 0x1FAFF || r >= 0x2600 && r <= 0x27BF
}

func CheckProseSentences(lines []string) []ProseProblem {
	return checkSentences(lines, false)
}

func checkSentences(lines []string, indented bool) []ProseProblem {
	var problems []ProseProblem
	inFence := false
	for i, raw := range lines {
		n := i + 1
		line := strings.TrimRight(raw, proseTrailing)
		if strings.Contains(line, proseEmDash) {
			problems = append(problems, ProseProblem{Line: n, Rule: ProseEmDash, Message: "the line has an em dash"})
		}
		if inFence {
			inFence = !IsFenceClose(fenceText(line, indented))
			continue
		}
		if IsFenceOpen(fenceText(line, indented)) {
			inFence = true
			continue
		}
		for _, text := range proseTexts(line) {
			if words := longestSentence(text); words > MaxProseSentenceWords {
				problems = append(problems, ProseProblem{Line: n, Rule: ProseSentenceLength, Message: fmt.Sprintf("sentence has %d words, the limit is %d", words, MaxProseSentenceWords)})
			}
		}
	}
	return problems
}

func CheckProse(lines []string, changed func(line int) bool) []ProseProblem {
	problems := checkSentences(lines, true)
	for i, line := range lines {
		if strings.IndexFunc(line, IsEmoji) >= 0 {
			problems = append(problems, ProseProblem{Line: i + 1, Rule: ProseEmoji, Message: "the line has an emoji"})
		}
	}
	var kept []ProseProblem
	for _, p := range problems {
		if changed(p.Line) {
			kept = append(kept, p)
		}
	}
	return append(kept, longParagraphs(lines, changed, true)...)
}

type paragraph struct {
	first   int
	text    []string
	item    bool
	changed bool
}

func longParagraphs(lines []string, changed func(line int) bool, indented bool) []ProseProblem {
	var problems []ProseProblem
	var cur *paragraph
	flush := func() {
		if cur != nil && cur.changed {
			if n := countSentences(strings.Join(cur.text, " ")); n > MaxProseParagraphSentences {
				problems = append(problems, ProseProblem{Line: cur.first, Rule: ProseParagraphLength, Message: fmt.Sprintf("paragraph has %d sentences, the limit is %d", n, MaxProseParagraphSentences)})
			}
		}
		cur = nil
	}
	add := func(n int, text string, item bool) {
		if cur == nil {
			cur = &paragraph{first: n, item: item}
		}
		cur.text = append(cur.text, text)
		cur.changed = cur.changed || changed(n)
	}
	inFence := false
	i := 0
	if len(lines) > 0 && strings.TrimRight(lines[0], proseTrailing) == proseFrontMatter {
		for i = 1; i < len(lines) && strings.TrimRight(lines[i], proseTrailing) != proseFrontMatter; i++ {
		}
		i++
	}
	for ; i < len(lines); i++ {
		n := i + 1
		line := strings.TrimRight(lines[i], proseTrailing)
		trimmed := strings.TrimSpace(line)
		switch {
		case inFence:
			inFence = !IsFenceClose(fenceText(line, indented))
		case IsFenceOpen(fenceText(line, indented)):
			flush()
			inFence = true
		case trimmed == "", strings.HasPrefix(trimmed, proseTableMark), strings.HasPrefix(line, proseHeadingMark):
			flush()
		case proseListMarker.MatchString(line):
			flush()
			add(n, proseListMarker.ReplaceAllString(line, ""), true)
		default:
			if cur != nil && cur.item && !proseIndented.MatchString(line) {
				flush()
			}
			add(n, trimmed, false)
		}
	}
	flush()
	return problems
}

func proseTexts(line string) []string {
	if !strings.HasPrefix(strings.TrimSpace(line), proseTableMark) {
		return []string{proseHeading.ReplaceAllString(proseListMarker.ReplaceAllString(line, ""), "")}
	}
	return TableCells(line)
}

func sentenceParts(text string) []string {
	return proseSentenceEnd.Split(MaskCodeSpans(text), -1)
}

func longestSentence(text string) int {
	longest := 0
	for _, sentence := range sentenceParts(text) {
		longest = max(longest, len(strings.Fields(sentence)))
	}
	return longest
}

func countSentences(text string) int {
	count := 0
	for _, sentence := range sentenceParts(text) {
		if len(strings.Fields(sentence)) > 0 {
			count++
		}
	}
	return count
}
