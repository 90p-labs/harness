package domain_test

import (
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

const goalSentence = "The service answers a ping with the word pong. The query records one span for each call."

func TestProblemExamples(t *testing.T) {
	valid := readValid(t)
	indent := strings.Repeat(" ", domain.TreeLevelWidth)
	header := "| " + strings.Join(domain.SignalsHeader, " | ") + " |"
	noSignals := edit{"## Signals\n", ""}
	tests := []struct {
		name  string
		edits []edit
		rule  domain.RuleID
		want  string
	}{
		{"tree indent", []edit{{"        │   └── pong.go ", "      │   └── pong.go "}}, domain.RuleTreeFormat,
			`indent must be 4 characters for each level. Example: "` + indent + `└── thing/" under "└── internal/"`},
		{"tree skip", []edit{{"            └── query/", "                    └── query/"}}, domain.RuleTreeFormat,
			`"query/" skips a level. A child is indented 4 more characters than its parent. A bad line above also causes this.`},
		{"sentence length", []edit{{goalSentence, words(26) + "."}}, domain.RuleSentenceLength,
			"sentence has 26 words, the limit is 25. Split it into two sentences."},
		{"em dash", []edit{{goalSentence, "The service answers a ping " + string(rune(0x2014)) + " with the word pong."}}, domain.RuleEmDash,
			"the line has an em dash. Use a comma or a full stop."},
		{"criterion unlinked", []edit{{"Criteria: 2 and 3", "Criteria: 2"}}, domain.RuleTraceUnlinked,
			`criterion 3 has no link. Name it in the Criteria: line of a section under ## Tech specs, such as "Criteria: 1 and 3"`},
		{"task unlinked", []edit{
			{"`internal/ping/domain/pong.go`, `internal/ping/domain/pong_test.go`", "`internal/ping/elsewhere/`"},
		}, domain.RuleTraceUnlinked,
			"task 1 has no link. List a final tree file in its Owned files, or add its owned file to a final tree"},
		{"signals missing", []edit{noSignals}, domain.RuleSignalsMissing,
			"the code tree adds a command, query, endpoint or consumer, so the plan needs ## Signals. Add ## Signals with the header " + header},
		{"signals table empty", []edit{{"| 2 | ping calls | prometheus | `ping:calls:rate5m` | `> 100` | `24h` | ping calls |\n", ""}}, domain.RuleSignalsTable,
			"## Signals needs a table with one row or more. Header: " + header},
		{"signals breach", []edit{{"`> 100`", "`big`"}}, domain.RuleSignalsTable,
			`breach "big" must be an operator (>, >=, <, <=) and a number. Example: "> 0.01"`},
		{"signals window", []edit{{"`24h`", "`soon`"}}, domain.RuleSignalsTable,
			`window "soon" is not a Go duration. Example: "30m" or "24h"`},
		{"signals criterion", []edit{{"| 2 | ping calls | prometheus |", "| two | ping calls | prometheus |"}}, domain.RuleSignalsTable,
			`signal criterion "two" is not a number. Example: "1"`},
		{"signals row cells", []edit{{"| `24h` | ping calls |\n", "| `24h` |\n"}}, domain.RuleSignalsTable,
			"row has 6 cells, want 7. Give one cell for each column of the header " + header},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := byRule(checkText(mutate(t, valid, tc.edits...), domain.CheckInput{}), tc.rule)
			if len(got) == 0 {
				t.Fatalf("no %s problem", tc.rule)
			}
			var messages []string
			for _, p := range got {
				if p.Message == tc.want {
					return
				}
				messages = append(messages, p.Message)
			}
			t.Errorf("messages = %q, want %q", messages, tc.want)
		})
	}
}

func TestTreeExampleParses(t *testing.T) {
	indent := strings.Repeat(" ", domain.TreeLevelWidth)
	text := "```tree\nharness/\n└── internal/\n" + indent + "└── thing/\n```\n"
	_, problems := domain.Parse(text)
	if got := byRule(problems, domain.RuleTreeFormat); len(got) != 0 {
		t.Errorf("tree example problems = %v, want none", got)
	}
}
