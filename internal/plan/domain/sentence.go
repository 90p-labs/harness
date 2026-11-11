package domain

import (
	shared "github.com/90p-labs/harness/internal/shared/domain"
)

const maxSentenceWords = shared.MaxProseSentenceWords

var proseRules = map[shared.ProseRule]RuleID{
	shared.ProseSentenceLength: RuleSentenceLength,
	shared.ProseEmDash:         RuleEmDash,
}

var proseFixes = map[shared.ProseRule]string{
	shared.ProseSentenceLength: "Split it into two sentences.",
	shared.ProseEmDash:         "Use a comma or a full stop.",
}

func checkSentences(doc Document, add report) {
	for _, p := range shared.CheckProseSentences(doc.Lines) {
		add(p.Line, proseRules[p.Rule], "%s. %s", p.Message, proseFixes[p.Rule])
	}
}
