package domain_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/plan/domain"
)

type edit [2]string

type fakeDiagrams struct {
	kinds []string
}

func (f *fakeDiagrams) Parse(kind, body string) error {
	f.kinds = append(f.kinds, kind)
	if strings.Contains(body, "BAD") {
		return errors.New("bad edge")
	}
	return nil
}

func mutate(t *testing.T, text string, edits ...edit) string {
	t.Helper()
	for _, e := range edits {
		if !strings.Contains(text, e[0]) {
			t.Fatalf("valid plan has no %q", e[0])
		}
		text = strings.Replace(text, e[0], e[1], 1)
	}
	return text
}

func checkText(text string, in domain.CheckInput) []domain.Problem {
	doc, problems := domain.Parse(text)
	problems = append(problems, domain.Check(doc, in)...)
	slices.SortStableFunc(problems, func(a, b domain.Problem) int {
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(string(a.Rule), string(b.Rule))
	})
	return problems
}

func byRule(problems []domain.Problem, rule domain.RuleID) []domain.Problem {
	var out []domain.Problem
	for _, p := range problems {
		if p.Rule == rule {
			out = append(out, p)
		}
	}
	return out
}

func TestCheckRules(t *testing.T) {
	valid := readValid(t)
	if got := checkText(valid, domain.CheckInput{Diagrams: &fakeDiagrams{}}); len(got) != 0 {
		t.Fatalf("valid plan has problems: %v", got)
	}

	longSentence := strings.TrimSpace(strings.Repeat("word ", 26)) + "."
	tests := []struct {
		name  string
		edits []edit
		want  []string
	}{
		{"heading missing", []edit{{"## Decisions\n", ""}}, []string{"plan.md:1: heading-missing:"}},
		{"final headings out of order", []edit{
			{"### Final code tree", "### @@"}, {"### Final test tree", "### Final code tree"}, {"### @@", "### Final test tree"},
		}, []string{"plan.md:84: heading-missing:"}},
		{"criteria header", []edit{{"| # | Criterion | Proof |", "| # | Text | Proof |"}}, []string{"plan.md:15: criteria-table:"}},
		{"criteria number", []edit{{"| 3 | The module", "| 4 | The module"}}, []string{
			"plan.md:19: criteria-table:", "plan.md:19: criterion-unowned:", "plan.md:19: trace-unlinked:",
			"plan.md:36: dod-unknown:", "plan.md:70: spec-criteria:",
		}},
		{"proof placeholder", []edit{{"`go test ./internal/ping/domain -run TestPong -count=1`", "`TBD`"}}, []string{"plan.md:17: criteria-table:"}},
		{"proof not a command", []edit{{"`make check`", "make check"}}, []string{"plan.md:19: criteria-table:"}},
		{"criterion unowned", []edit{{"criteria 2 to 3.", "criterion 2."}}, []string{"plan.md:19: criterion-unowned:"}},
		{"dod unknown", []edit{{"criteria 2 to 3.", "criteria 2 to 4."}}, []string{"plan.md:36: dod-unknown:"}},
		{"task field missing", []edit{{"- Side: backend\n", ""}}, []string{"plan.md:23: task-field:"}},
		{"task side unknown", []edit{{"- Side: backend", "- Side: middle"}}, []string{"plan.md:25: task-field:"}},
		{"depends unknown task", []edit{{"- Depends on: task 1", "- Depends on: task 9"}}, []string{"plan.md:37: task-depends:"}},
		{"depends cycle", []edit{{"- Depends on: none", "- Depends on: task 2"}}, []string{"plan.md:29: task-depends:"}},
		{"tree mark missing", []edit{{"pong.go                    new", "pong.go"}}, []string{"plan.md:91: tree-format:"}},
		{"tree file unowned", []edit{{"pong.go                    new", "extra.go                   new"}}, []string{"plan.md:91: tree-unowned:"}},
		{"commit not conventional", []edit{{"`feat: add the pong domain`", "`add the pong domain`"}}, []string{"plan.md:114: commit-format:"}},
		{"commit unknown task", []edit{{"(task 1)", "(task 7)"}}, []string{"plan.md:114: commit-format:"}},
		{"task uncommitted", []edit{{"2. `feat: add the ping query` (task 2)\n", ""}, {"(tasks 1 and 2)", "(task 1)"}}, []string{"plan.md:31: task-uncommitted:"}},
		{"no run PR", []edit{{" (this run)", ""}}, []string{"plan.md:118: pr-sequence:"}},
		{"two run PRs", []edit{{"`docs/ping-flow`:", "`docs/ping-flow` (this run):"}}, []string{"plan.md:121: pr-sequence:"}},
		{"sentence too long", []edit{{"The domain holds the word that the query returns.", longSentence}}, []string{"plan.md:57: sentence-length:"}},
		{"em dash", []edit{{"The domain holds", "The domain " + string(rune(0x2014)) + " holds"}}, []string{"plan.md:57: em-dash:"}},
		{"signals missing", []edit{{"## Signals\n", ""}}, []string{"plan.md:83: signals-missing:"}},
		{"signals header", []edit{{"| Threshold | Window |", "| Threshold | Time |"}}, []string{"plan.md:41: signals-table:"}},
		{"signals window", []edit{{"`24h`", "`soon`"}}, []string{"plan.md:43: signals-table:"}},
		{"diagram does not parse", []edit{{"Handler --> Pong", "Handler BAD Pong"}}, []string{"plan.md:59: diagram:"}},
		{"spec names unknown criterion", []edit{{"Criteria: 1\n", "Criteria: 1 and 9\n"}}, []string{"plan.md:52: spec-criteria:"}},
		{"criterion no spec", []edit{{"Criteria: 2 and 3", "Criteria: 2"}}, []string{"plan.md:19: trace-unlinked:"}},
		{"signals without a table", []edit{{"| Criterion | Metric | Panel | Alert | Threshold | Window |\n|---|---|---|---|---|---|\n| 2 | `ping.query` span count | ping calls | none | none | `24h` |\n", ""}}, []string{"plan.md:39: signals-table:"}},
		{"signals table without a row", []edit{{"| 2 | `ping.query` span count | ping calls | none | none | `24h` |\n", ""}}, []string{"plan.md:39: signals-table:"}},
		{"duplicate task number", []edit{{"### 2. Ping query", "### 1. Ping query"}, {"(task 2)", "(task 1)"}, {"(tasks 1 and 2)", "(task 1)"}}, []string{"plan.md:31: task-field:"}},
		{"proof with a dotted package path", []edit{{"`make check`", "`go test ./... -count=1`"}}, nil},
		{"proof with redirections", []edit{{"`make check`", "`sort < in.txt > out.txt`"}}, nil},
		{"proof with an angle placeholder", []edit{{"`make check`", "`go test <package>`"}}, []string{"plan.md:19: criteria-table:"}},
		{"proof with a spaced angle placeholder", []edit{{"`make check`", "`go test <the package>`"}}, []string{"plan.md:19: criteria-table:"}},
		{"proof with a path placeholder", []edit{{"`make check`", "`cat <path/to/file>`"}}, []string{"plan.md:19: criteria-table:"}},
		{"proof that is only an ellipsis", []edit{{"`make check`", "`...`"}}, []string{"plan.md:19: criteria-table:"}},
		{"proof with spaced redirections", []edit{{"`make check`", "`cmd < in > out`"}}, nil},
		{"owned path with a dot slash", []edit{{"`internal/ping/domain/pong.go`, ", "`./internal/ping/domain/pong.go`, "}}, nil},
		{"depends with a full stop", []edit{{"- Depends on: task 1", "- Depends on: task 1."}}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := checkText(mutate(t, valid, tc.edits...), domain.CheckInput{Diagrams: &fakeDiagrams{}})
			var lines []string
			for _, p := range got {
				lines = append(lines, p.String())
			}
			if len(lines) != len(tc.want) {
				t.Fatalf("problems = %q, want prefixes %q", lines, tc.want)
			}
			for i, prefix := range tc.want {
				if !strings.HasPrefix(lines[i], prefix) {
					t.Errorf("problem %d = %q, want prefix %q", i, lines[i], prefix)
				}
			}
		})
	}
}

func TestDiagramRule(t *testing.T) {
	valid := readValid(t)
	bad := mutate(t, valid, edit{"Handler --> Pong", "Handler BAD Pong"})
	unknownKind := mutate(t, valid, edit{"classDiagram\n", "flowchart TD\n"}, edit{"Handler --> Pong", "Handler BAD Pong"})

	fake := &fakeDiagrams{}
	if got := checkText(unknownKind, domain.CheckInput{Diagrams: fake}); len(got) != 0 {
		t.Errorf("unknown kind has problems: %v", got)
	}
	if want := []string{domain.DiagramState}; !slices.Equal(fake.kinds, want) {
		t.Errorf("parsed kinds = %v, want %v", fake.kinds, want)
	}
	if got := checkText(bad, domain.CheckInput{}); len(got) != 0 {
		t.Errorf("nil parser has problems: %v", got)
	}
}

func TestSignalsRequired(t *testing.T) {
	valid := readValid(t)
	noSignals := edit{"## Signals\n", ""}
	tests := []struct {
		name  string
		edits []edit
		want  int
	}{
		{"query file with the table", nil, 0},
		{"query file without the table", []edit{noSignals}, 1},
		{"command file without the table", []edit{noSignals, {"└── query/", "└── command/"}}, 1},
		{"http file without the table", []edit{noSignals, {"        └── application/\n            └── query/", "        └── infrastructure/\n            └── http/"}}, 1},
		{"domain file only without the table", []edit{noSignals, {"└── query/", "└── lookup/"}}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := byRule(checkText(mutate(t, valid, tc.edits...), domain.CheckInput{}), domain.RuleSignalsMissing)
			if len(got) != tc.want {
				t.Errorf("signals-missing = %v, want %d", got, tc.want)
			}
		})
	}
}

func TestRegressionImpact(t *testing.T) {
	valid := readValid(t)
	section := func(body string) edit {
		return edit{"## Tech specs", "## Regression impact\n\n- " + body + "\n\n## Tech specs"}
	}
	tests := []struct {
		name  string
		edits []edit
		in    domain.CheckInput
		want  int
	}{
		{"round 0 needs no section", nil, domain.CheckInput{ArchivedPlans: []string{"plan-1.md"}}, 0},
		{"round 1 without the section", nil, domain.CheckInput{LiveRound: 1, ArchivedPlans: []string{"plan-1.md"}}, 1},
		{"round 1 names an archived plan", []edit{section("The ping flow is in `plan-1.md`.")}, domain.CheckInput{LiveRound: 1, ArchivedPlans: []string{"plan-1.md"}}, 0},
		{"round 1 names a plan that does not exist", []edit{section("The ping flow is in `plan-9.md`.")}, domain.CheckInput{LiveRound: 1, ArchivedPlans: []string{"plan-1.md"}}, 1},
		{"round 1 names no plan", []edit{section("The ping flow stays.")}, domain.CheckInput{LiveRound: 1, ArchivedPlans: []string{"plan-1.md"}}, 1},
		{"round 1 with no archived plan", []edit{section("The ping flow is in `plan-1.md`.")}, domain.CheckInput{LiveRound: 1}, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := byRule(checkText(mutate(t, valid, tc.edits...), tc.in), domain.RuleRegressionImpact)
			if len(got) != tc.want {
				t.Errorf("regression-impact = %v, want %d", got, tc.want)
			}
		})
	}
}
