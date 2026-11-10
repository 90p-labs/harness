package domain

import (
	shared "github.com/90p-labs/harness/internal/shared/domain"
)

const maxSentenceWords = shared.MaxProseSentenceWords

var proseRules = map[shared.ProseRule]RuleID{
	shared.ProseSentenceLength: RuleSentenceLength,
	shared.ProseEmDash:         RuleEmDash,
}

func checkSentences(doc Document, add report) {
	for _, p := range shared.CheckProseSentences(doc.Lines) {
		add(p.Line, proseRules[p.Rule], "%s", p.Message)
	}
}
