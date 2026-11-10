package domain_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

const (
	noRisks        = "## Review risks\n\n- none\n"
	openQuestions  = "## Open questions"
	codeTreeDomain = "        ├── domain/\n        │   └── pong.go"
)

func risks(body string) edit {
	return edit{noRisks, "## Review risks\n\n" + body}
}

func guardPath(folder string) edit {
	return edit{codeTreeDomain, "        ├── " + folder + "/\n        │   └── pong.go"}
}

func threat(body string) edit {
	return edit{openQuestions, "## Threat model\n\n" + body + "\n" + openQuestions}
}

func TestReviewRisks(t *testing.T) {
	valid := readValid(t)
	tests := []struct {
		name  string
		edits []edit
		want  []domain.RuleID
	}{
		{"none", nil, nil},
		{"heading missing", []edit{{noRisks, ""}}, []domain.RuleID{domain.RuleHeadingMissing}},
		{"test in a proof", []edit{risks("- Late reply. Test: `TestPong`.\n")}, nil},
		{"test in a test first line", []edit{risks("- Late reply. Test: `TestPingQuery`.\n")}, nil},
		{"decision", []edit{risks("- Late reply. Decision: the handler retries once.\n")}, nil},
		{"neither label", []edit{risks("- Late reply.\n")}, []domain.RuleID{domain.RuleRiskUnanchored}},
		{"test without backticks", []edit{risks("- Late reply. Test: TestPong.\n")}, []domain.RuleID{domain.RuleRiskUnanchored}},
		{"decision without text", []edit{risks("- Late reply. Decision:\n")}, []domain.RuleID{domain.RuleRiskUnanchored}},
		{"test in no proof", []edit{risks("- Late reply. Test: `TestGhost`.\n")}, []domain.RuleID{domain.RuleRiskUnanchored}},
		{"test name is a prefix of a proof name", []edit{risks("- Late reply. Test: `TestPin`.\n")}, []domain.RuleID{domain.RuleRiskUnanchored}},
		{"line without a bullet", []edit{risks("Late reply.\n")}, []domain.RuleID{domain.RuleRiskUnanchored, domain.RuleRiskUnanchored}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			problems := checkText(mutate(t, valid, tc.edits...), domain.CheckInput{Diagrams: &fakeDiagrams{}})

			var got []domain.RuleID
			for _, p := range problems {
				got = append(got, p.Rule)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.want) {
				t.Fatalf("rules = %v, want %v (problems %v)", got, tc.want, problems)
			}
		})
	}
}

func TestReviewRisksNamesTheTest(t *testing.T) {
	text := mutate(t, readValid(t), risks("- Late reply. Test: `TestGhost`.\n"))

	got := byRule(checkText(text, domain.CheckInput{Diagrams: &fakeDiagrams{}}), domain.RuleRiskUnanchored)

	if len(got) != 1 || !strings.Contains(got[0].Message, "TestGhost") {
		t.Fatalf("problems = %v, want one that names TestGhost", got)
	}
}

func TestThreatModel(t *testing.T) {
	valid := readValid(t)
	complete := "- Attacker: an agent.\n- Input forms: a line.\n- Fail closed: ask.\n"
	tests := []struct {
		name  string
		edits []edit
		want  []string
	}{
		{"no guard path and no section", nil, nil},
		{"guard path without the section", []edit{guardPath("guard")}, []string{domain.HeadingThreatModel}},
		{"security path without the section", []edit{guardPath("security")}, []string{domain.HeadingThreatModel}},
		{"guardrail does not trigger", []edit{guardPath("guardrail")}, nil},
		{"guard path with the section", []edit{guardPath("guard"), threat(complete)}, nil},
		{"no Attacker", []edit{guardPath("guard"), threat("- Input forms: a line.\n- Fail closed: ask.\n")}, []string{domain.ThreatAttacker}},
		{"no Input forms", []edit{guardPath("guard"), threat("- Attacker: an agent.\n- Fail closed: ask.\n")}, []string{domain.ThreatInputForms}},
		{"no Fail closed", []edit{guardPath("guard"), threat("- Attacker: an agent.\n- Input forms: a line.\n")}, []string{domain.ThreatFailClosed}},
		{"bullet with no text", []edit{guardPath("guard"), threat("- Attacker:\n- Input forms: a line.\n- Fail closed: ask.\n")}, []string{domain.ThreatAttacker}},
		{"section on a plan with no guard path", []edit{threat("- Attacker: an agent.\n")}, []string{domain.ThreatInputForms, domain.ThreatFailClosed}},
		{"complete section on a plan with no guard path", []edit{threat(complete)}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := byRule(checkText(mutate(t, valid, tc.edits...), domain.CheckInput{Diagrams: &fakeDiagrams{}}), domain.RuleThreatModel)

			if len(got) != len(tc.want) {
				t.Fatalf("problems = %v, want %d naming %v", got, len(tc.want), tc.want)
			}
			for i, w := range tc.want {
				if !strings.Contains(got[i].Message, w) {
					t.Errorf("problem %d = %q, want it to name %q", i, got[i].Message, w)
				}
			}
		})
	}
}

func extraTasks(count int) string {
	var b strings.Builder
	for n := 3; n < 3+count; n++ {
		fmt.Fprintf(&b, "### %d. Extra %d\n\n- Side: backend\n- Owned files: `internal/ping/domain/pong.go`\n- Test first: `TestPong`\n- DoD: criterion 1.\n- Depends on: none\n\n", n, n)
	}
	return b.String()
}

func TestPlanSize(t *testing.T) {
	valid := readValid(t)
	tests := []struct {
		name  string
		extra int
		fails bool
	}{
		{"2 tasks", 0, false},
		{"6 tasks", domain.MaxRunTasks - 2, false},
		{"7 tasks", domain.MaxRunTasks - 1, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := mutate(t, valid, edit{"## Signals", extraTasks(tc.extra) + "## Signals"})

			got := byRule(checkText(text, domain.CheckInput{Diagrams: &fakeDiagrams{}}), domain.RulePlanSize)

			if (len(got) == 1) != tc.fails || len(got) > 1 {
				t.Fatalf("problems = %v, fails want %v", got, tc.fails)
			}
			if tc.fails && !strings.Contains(got[0].Message, "multi-run plan") {
				t.Errorf("message %q does not tell to split into a multi-run plan", got[0].Message)
			}
		})
	}
}
