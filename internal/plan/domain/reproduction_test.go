package domain_test

import (
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

const reproductionSection = "## Reproduction\n\n" +
	"- Criterion: 1\n" +
	"- Symptom: `want pong, got ping`\n" +
	"- Regression test: `TestPong`\n" +
	"\n1. If the cache is stale, then a rerun passes. Result: refuted.\n" +
	"2. If the domain returns the wrong word, then the test shows ping. Result: confirmed.\n" +
	"3. If the query drops the word, then the span is empty. Result: untested.\n"

func bugPlan(t *testing.T, edits ...edit) string {
	t.Helper()
	text := mutate(t, readValid(t),
		edit{"Track: full\n", "Track: full\nCause: bug\n"},
		edit{"## Tasks\n", reproductionSection + "\n## Tasks\n"},
	)
	return mutate(t, text, edits...)
}

func reproductionProblems(text string) []string {
	var lines []string
	for _, p := range byRule(checkText(text, domain.CheckInput{Diagrams: &fakeDiagrams{}}), domain.RuleReproduction) {
		lines = append(lines, p.String())
	}
	return lines
}

func TestReproductionCauseIsCaseInsensitive(t *testing.T) {
	doc, problems := domain.Parse(bugPlan(t, edit{"Cause: bug", "Cause: Bug"}))

	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	if doc.Cause != domain.CauseBug {
		t.Errorf("cause = %q, want %q", doc.Cause, domain.CauseBug)
	}
	if got := reproductionProblems(bugPlan(t, edit{"Cause: bug", "Cause: Bug"})); len(got) != 0 {
		t.Errorf("problems = %v, want none", got)
	}
}

func TestReproductionRuleParse(t *testing.T) {
	doc, problems := domain.Parse(bugPlan(t))

	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	if doc.Cause != domain.CauseBug || doc.CauseLine != 8 {
		t.Errorf("cause = %q on line %d, want bug on line 8", doc.Cause, doc.CauseLine)
	}
	r := doc.Reproduction
	if r.Line == 0 || r.Criterion.Value != "1" || r.Symptom.Value != "`want pong, got ping`" || r.Regression.Value != "`TestPong`" {
		t.Errorf("reproduction = %+v", r)
	}
	if r.HumanOnly.Present() {
		t.Errorf("human only = %+v, want absent", r.HumanOnly)
	}
	if len(r.Hypotheses) != 3 || r.Hypotheses[1].Result != "confirmed" || !r.Hypotheses[1].Formed || r.Hypotheses[2].Number != 3 {
		t.Errorf("hypotheses = %+v", r.Hypotheses)
	}
	plain, _ := domain.Parse(readValid(t))
	if plain.Cause != "" || plain.Reproduction.Line != 0 {
		t.Errorf("a plan with no cause has cause %q and section line %d", plain.Cause, plain.Reproduction.Line)
	}
}

func TestReproductionSecondHeading(t *testing.T) {
	text := bugPlan(t, edit{"## Tasks\n", "## Reproduction\n\n## Tasks\n"})
	before, _, _ := strings.Cut(text, "## Reproduction\n\n## Tasks")
	want := strings.Count(before, "\n") + 1

	got := byRule(checkText(text, domain.CheckInput{Diagrams: &fakeDiagrams{}}), domain.RuleReproduction)

	if len(got) != 1 || got[0].Line != want {
		t.Fatalf("problems = %v, want one at line %d", got, want)
	}
}

func TestReproductionRuleProblems(t *testing.T) {
	humanOnly := edit{"- Criterion: 1\n- Symptom: `want pong, got ping`\n", "- Human only: the bug needs a phone\n"}
	tests := []struct {
		name  string
		text  string
		lines []string
	}{
		{"bug plan with a reproduction", bugPlan(t), nil},
		{"human only", bugPlan(t, humanOnly), nil},
		{"human only with untested hypotheses", bugPlan(t, humanOnly, edit{"Result: confirmed", "Result: untested"}), nil},
		{"plan with no cause", readValid(t), nil},
		{"other cause", bugPlan(t, edit{"Cause: bug", "Cause: typo"}), []string{"plan.md:8: reproduction:"}},
		{"cause with no section", mutate(t, readValid(t), edit{"Track: full\n", "Track: full\nCause: bug\n"}), []string{"plan.md:8: reproduction:"}},
		{"section with no cause", bugPlan(t, edit{"Cause: bug\n", ""}), []string{"plan.md:"}},
		{"criterion and human only", bugPlan(t, edit{"- Regression test", "- Human only: a phone\n- Regression test"}), []string{"plan.md:"}},
		{"neither criterion nor human only", bugPlan(t, edit{"- Criterion: 1\n- Symptom: `want pong, got ping`\n", ""}), []string{"plan.md:"}},
		{"unknown criterion", bugPlan(t, edit{"- Criterion: 1", "- Criterion: 9"}), []string{"plan.md:"}},
		{"symptom missing", bugPlan(t, edit{"- Symptom: `want pong, got ping`\n", ""}), []string{"plan.md:"}},
		{"symptom without backticks", bugPlan(t, edit{"`want pong, got ping`", "want pong, got ping"}), []string{"plan.md:"}},
		{"symptom with two texts", bugPlan(t, edit{"`want pong, got ping`", "`want pong` and `got ping`"}), []string{"plan.md:"}},
		{"symptom with human only", bugPlan(t, edit{"- Regression test", "- Human only: a phone\n- Regression test"}, edit{"- Criterion: 1\n", ""}), []string{"plan.md:"}},
		{"short symptom", bugPlan(t, edit{"`want pong, got ping`", "`short`"}), []string{"plan.md:"}},
		{"symptom in the proof command", bugPlan(t, edit{"`want pong, got ping`", "`-run TestPong`"}), []string{"plan.md:"}},
		{"exit status symptom", bugPlan(t, edit{"`want pong, got ping`", "`Exit Status 1 and more`"}), []string{"plan.md:"}},
		{"fail symptom", bugPlan(t, edit{"`want pong, got ping`", "`--- FAIL: TestPong`"}), []string{"plan.md:"}},
		{"fail colon symptom", bugPlan(t, edit{"`want pong, got ping`", "`FAIL: TestPong`"}), []string{"plan.md:"}},
		{"fail space symptom", bugPlan(t, edit{"`want pong, got ping`", "`FAIL TestPong`"}), []string{"plan.md:"}},
		{"fail tab symptom", bugPlan(t, edit{"`want pong, got ping`", "`FAIL\tTestPong`"}), []string{"plan.md:"}},
		{"failed word symptom", bugPlan(t, edit{"`want pong, got ping`", "`failed to parse the config`"}), nil},
		{"build failed symptom", bugPlan(t, edit{"`want pong, got ping`", "`BUILD FAILED in a step`"}), []string{"plan.md:"}},
		{"two hypotheses", bugPlan(t, edit{"3. If the query drops the word, then the span is empty. Result: untested.\n", ""}), []string{"plan.md:"}},
		{"six hypotheses", bugPlan(t, edit{"3. If the query drops the word, then the span is empty. Result: untested.\n", "3. If a, then b. Result: refuted.\n4. If c, then d. Result: refuted.\n5. If e, then f. Result: refuted.\n6. If g, then h. Result: refuted.\n"}), []string{"plan.md:"}},
		{"hypothesis out of form", bugPlan(t, edit{"3. If the query drops the word, then the span is empty. Result: untested.", "3. The query drops the word."}), []string{"plan.md:"}},
		{"hypothesis with another result word", bugPlan(t, edit{"Result: untested", "Result: maybe"}), []string{"plan.md:"}},
		{"hypothesis numbered out of order", bugPlan(t, edit{"3. If the query", "4. If the query"}), []string{"plan.md:"}},
		{"no confirmed hypothesis", bugPlan(t, edit{"Result: confirmed", "Result: refuted"}), []string{"plan.md:"}},
		{"untested above the confirmed one", bugPlan(t, edit{"Result: refuted", "Result: untested"}), []string{"plan.md:"}},
		{"regression test missing", bugPlan(t, edit{"- Regression test: `TestPong`\n", ""}), []string{"plan.md:"}},
		{"regression test in no proof", bugPlan(t, edit{"- Regression test: `TestPong`", "- Regression test: `TestNowhere`"}), []string{"plan.md:"}},
		{"regression test in a proof", bugPlan(t, edit{"- Regression test: `TestPong`", "- Regression test: `TestPingQuery`"}), nil},
		{"regression test in a test first line", bugPlan(t, edit{"- Regression test: `TestPong`", "- Regression test: `TestOnlyFirst`"}, edit{"`TestPong` fails, because", "`TestOnlyFirst` fails, because"}), nil},
		{"regression none with a reason", bugPlan(t, edit{"- Regression test: `TestPong`", "- Regression test: none: no seam exists for the clock"}), nil},
		{"regression none with no reason", bugPlan(t, edit{"- Regression test: `TestPong`", "- Regression test: none:"}), []string{"plan.md:"}},
		{"fix commit with a cause", bugPlan(t, edit{"`feat: add the ping query`", "`fix: add the ping query`"}), nil},
		{"fix commit with no cause", mutate(t, readValid(t), edit{"`feat: add the ping query`", "`fix: add the ping query`"}), []string{"plan.md:115: reproduction:"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := reproductionProblems(tc.text)
			if len(got) != len(tc.lines) {
				t.Fatalf("problems = %q, want prefixes %q", got, tc.lines)
			}
			for i, prefix := range tc.lines {
				if !strings.HasPrefix(got[i], prefix) {
					t.Errorf("problem %d = %q, want prefix %q", i, got[i], prefix)
				}
			}
		})
	}
}
