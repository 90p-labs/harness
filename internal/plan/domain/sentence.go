package domain

import (
	"regexp"
	"strings"
)

const (
	maxSentenceWords = 25
	emDash           = string(rune(0x2014))
	codeSpanWord     = "code"
)

var (
	codeSpan    = regexp.MustCompile("`[^`]*`")
	sentenceEnd = regexp.MustCompile(`[.!?]+(?:\s+|$)`)
	listMarker  = regexp.MustCompile(`^\s*(?:[-*]|\d+\.)\s+`)
	headingMark = regexp.MustCompile(`^#+\s+`)
)

func checkSentences(doc Document, add report) {
	inFence := false
	for i, raw := range doc.Lines {
		n := i + 1
		line := strings.TrimRight(raw, trailingSpace)
		if strings.Contains(line, emDash) {
			add(n, RuleEmDash, "the line has an em dash")
		}
		if inFence {
			inFence = !closesFence(line)
			continue
		}
		if opensFence(line) {
			inFence = true
			continue
		}
		for _, text := range proseTexts(line) {
			if words := longestSentence(text); words > maxSentenceWords {
				add(n, RuleSentenceLength, "sentence has %d words, the limit is %d", words, maxSentenceWords)
			}
		}
	}
}

func proseTexts(line string) []string {
	if !strings.HasPrefix(strings.TrimSpace(line), "|") {
		return []string{headingMark.ReplaceAllString(listMarker.ReplaceAllString(line, ""), "")}
	}
	c := cells(line)
	if isSeparator(c) {
		return nil
	}
	return c
}

func longestSentence(text string) int {
	longest := 0
	for _, sentence := range sentenceEnd.Split(codeSpan.ReplaceAllString(text, codeSpanWord), -1) {
		longest = max(longest, len(strings.Fields(sentence)))
	}
	return longest
}
