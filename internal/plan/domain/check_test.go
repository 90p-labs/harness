package domain_test

import (
	"errors"
	"fmt"
	"os"
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

const (
	signalsHeaderLines = "| Criterion | Signal | Source | Query | Breach | Window | Panel |\n|---|---|---|---|---|---|---|\n"
	signalsRow         = "| 2 | ping calls | prometheus | `ping:calls:rate5m` | `> 100` | `24h` | ping calls |\n"
)

func TestCheckSignalsTable(t *testing.T) {
	valid := readValid(t)
	tests := []struct {
		name  string
		edits []edit
		want  []string
	}{
		{"valid row", nil, nil},
		{"bad breach operator", []edit{{"`> 100`", "`= 100`"}}, []string{"plan.md:43: signals-table:"}},
		{"breach not a number", []edit{{"`> 100`", "`> many`"}}, []string{"plan.md:43: signals-table:"}},
		{"bad window", []edit{{"`24h`", "`soon`"}}, []string{"plan.md:43: signals-table:"}},
		{"bad breach and bad window", []edit{{"`> 100`", "`100`"}, {"`24h`", "`soon`"}}, []string{"plan.md:43: signals-table:", "plan.md:43: signals-table:"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := byRule(checkText(mutate(t, valid, tc.edits...), domain.CheckInput{Diagrams: &fakeDiagrams{}}), domain.RuleSignalsTable)
			if len(got) != len(tc.want) {
				t.Fatalf("problems = %v, want prefixes %q", got, tc.want)
			}
			for i, prefix := range tc.want {
				if !strings.HasPrefix(got[i].String(), prefix) {
					t.Errorf("problem %d = %q, want prefix %q", i, got[i], prefix)
				}
			}
		})
	}
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
		{"signals header", []edit{{"| Breach | Window |", "| Breach | Time |"}}, []string{"plan.md:41: signals-table:"}},
		{"diagram does not parse", []edit{{"Handler --> Pong", "Handler BAD Pong"}}, []string{"plan.md:59: diagram:"}},
		{"diagram unmarked", []edit{{"Ready --> Answered : ping (new)", "Ready --> Answered : ping"}}, []string{"plan.md:77: diagram:"}},
		{"spec names unknown criterion", []edit{{"Criteria: 1\n", "Criteria: 1 and 9\n"}}, []string{"plan.md:52: spec-criteria:"}},
		{"criterion no spec", []edit{{"Criteria: 2 and 3", "Criteria: 2"}}, []string{"plan.md:19: trace-unlinked:"}},
		{"signals without a table", []edit{{signalsHeaderLines + signalsRow, ""}}, []string{"plan.md:39: signals-table:"}},
		{"signals table without a row", []edit{{signalsRow, ""}}, []string{"plan.md:39: signals-table:"}},
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

	unmarked := mutate(t, valid, edit{"    Ready --> Answered : ping (new)\n", "    Ready --> Answered : ping\n"})
	restyled := mutate(t, valid, edit{"classDef new stroke:#00b34e,stroke-width:3px,color:#00b34e", "classDef new stroke:#000"})
	flowchart := "# Plan\n\n```mermaid\nflowchart TD\n    classDef new stroke:#00b34e,stroke-width:3px,color:#00b34e\n    a --> b\n    class a new\n    cssClass \"b\" new\n```\n"

	t.Run("an unmarked diagram is refused with no parser", func(t *testing.T) {
		got := byRule(checkText(unmarked, domain.CheckInput{}), domain.RuleDiagram)
		if len(got) != 1 || got[0].Line != 77 || !strings.Contains(got[0].Message, "no mark") {
			t.Errorf("problems = %v, want one no mark problem at line 77", got)
		}
	})
	t.Run("a restyled mark is refused", func(t *testing.T) {
		got := byRule(checkText(restyled, domain.CheckInput{Diagrams: &fakeDiagrams{}}), domain.RuleDiagram)
		if len(got) != 1 || got[0].Line != 59 || !strings.Contains(got[0].Message, "another style") {
			t.Errorf("problems = %v, want one another style problem at line 59", got)
		}
	})
	t.Run("a flowchart with class and cssClass lines stays clean", func(t *testing.T) {
		if got := byRule(checkText(flowchart, domain.CheckInput{Diagrams: &fakeDiagrams{}}), domain.RuleDiagram); len(got) != 0 {
			t.Errorf("problems = %v, want none", got)
		}
	})
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

func lineOf(t *testing.T, text, needle string) int {
	t.Helper()
	for i, l := range strings.Split(text, "\n") {
		if strings.Contains(l, needle) {
			return i + 1
		}
	}
	t.Fatalf("text has no line with %q", needle)
	return 0
}

func TestDiagramRequiredRule(t *testing.T) {
	raw, err := os.ReadFile("testdata/diagrams-missing.md")
	if err != nil {
		t.Fatal(err)
	}
	missing := string(raw)
	wantFile := []struct{ at, msg string }{
		{"![The list]", "sketch ui/list.excalidraw is not a file"},
		{"001_pings.sql", "erDiagram"},
		{"clock.go", "3 new files in domain/ need a classDiagram"},
		{"handler.go", "sequenceDiagram"},
		{"openapi.yaml", "classDiagram"},
		{"openapi.yaml", "sequenceDiagram"},
	}

	got := byRule(checkText(missing, domain.CheckInput{}), domain.RuleDiagramRequired)

	if len(got) != len(wantFile) {
		t.Fatalf("diagram-required = %v, want %d problems", got, len(wantFile))
	}
	for i, w := range wantFile {
		if got[i].Line != lineOf(t, missing, w.at) || !strings.Contains(got[i].Message, w.msg) {
			t.Errorf("problem %d = %v, want line of %q and %q", i, got[i], w.at, w.msg)
		}
	}

	const (
		sketchPath = "ui/x.excalidraw"
		sketchLink = " ![x](" + sketchPath + ")"
		pongLine   = "└── pong.go                    new"
		httpLines  = "        ├── infrastructure/\n        │   └── http/\n        │       └── ping.go                %s\n"
		manyFiles  = "├── pong.go                    new\n        │   ├── a.go                       new\n        │   └── b.go                       new"
	)
	withDiagram := func(kind, body string) edit {
		return edit{"### Final code tree", "```mermaid\n" + kind + "\n    " + body + "\n```\n\n### Final code tree"}
	}
	withoutClass := edit{"classDiagram\n", "flowchart TD\n"}
	tests := []struct {
		name     string
		edits    []edit
		sketches []string
		at, msg  string
	}{
		{"sql file with no erDiagram", []edit{{pongLine, "└── 001.sql                    new"}}, nil, "001.sql", "erDiagram"},
		{"sql file with an erDiagram", []edit{{pongLine, "└── 001.sql                    new"}, withDiagram("erDiagram", "a ||--o{ b : has")}, nil, "", ""},
		{"migrations folder with no erDiagram", []edit{{pongLine, "└── migrations/\n        │       └── 001.txt            changed"}}, nil, "001.txt", "erDiagram"},
		{"proto file with no classDiagram", []edit{withoutClass, {pongLine, "└── ping.proto                 changed"}}, nil, "ping.proto", "classDiagram"},
		{"proto file with a classDiagram", []edit{{pongLine, "└── ping.proto                 changed"}}, nil, "", ""},
		{"graphql file", []edit{withoutClass, {pongLine, "└── ping.graphql               changed"}}, nil, "ping.graphql", "classDiagram"},
		{"infix openapi file", []edit{withoutClass, {pongLine, "└── ping.openapi.json          changed"}}, nil, "ping.openapi.json", "classDiagram"},
		{"asyncapi file", []edit{withoutClass, {pongLine, "└── asyncapi.yaml              changed"}}, nil, "asyncapi.yaml", "classDiagram"},
		{"new http file with no sequenceDiagram", []edit{{"        └── application/", fmt.Sprintf(httpLines, "new") + "        └── application/"}}, nil, "ping.go", "sequenceDiagram"},
		{"new http file with a sequenceDiagram", []edit{{"        └── application/", fmt.Sprintf(httpLines, "new") + "        └── application/"}, withDiagram("sequenceDiagram", "A->>B: go")}, nil, "", ""},
		{"new http proto file with a classDiagram", []edit{{"        └── application/", "        ├── infrastructure/\n        │   └── http/\n        │       └── api.proto              new\n        └── application/"}}, nil, "api.proto", "sequenceDiagram"},
		{"changed http file", []edit{{"        └── application/", fmt.Sprintf(httpLines, "changed") + "        └── application/"}}, nil, "", ""},
		{"three new domain files with no classDiagram", []edit{withoutClass, {pongLine, manyFiles}}, nil, "b.go", "3 new files in domain/"},
		{"three new domain files with a classDiagram", []edit{{pongLine, manyFiles}}, nil, "", ""},
		{"two new domain files", []edit{withoutClass, {pongLine, "├── pong.go                    new\n        │   └── a.go                       new"}}, nil, "", ""},
		{"frontend task with no sketch link", []edit{{"- Side: backend\n- Owned files: `internal/ping/application", "- Side: frontend\n- Owned files: `internal/ping/application"}}, nil, "### 2. Ping query", "task 2 is frontend"},
		{"frontend task with a sketch link", []edit{{"- Side: backend\n- Owned files: `internal/ping/application", "- Side: frontend\n- Owned files: `internal/ping/application"}, {"returns.\n", "returns." + sketchLink + "\n"}}, []string{sketchPath}, "", ""},
		{"sketch link with no file", []edit{{"returns.\n", "returns." + sketchLink + "\n"}}, nil, "![x]", "sketch ui/x.excalidraw is not a file"},
		{"sketch link with a file", []edit{{"returns.\n", "returns." + sketchLink + "\n"}}, []string{sketchPath}, "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := mutate(t, readValid(t), tc.edits...)

			got := byRule(checkText(text, domain.CheckInput{Sketches: tc.sketches}), domain.RuleDiagramRequired)

			if tc.at == "" {
				if len(got) != 0 {
					t.Fatalf("diagram-required = %v, want none", got)
				}
				return
			}
			if len(got) != 1 || got[0].Line != lineOf(t, text, tc.at) || !strings.Contains(got[0].Message, tc.msg) {
				t.Fatalf("diagram-required = %v, want one on the line of %q with %q", got, tc.at, tc.msg)
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
