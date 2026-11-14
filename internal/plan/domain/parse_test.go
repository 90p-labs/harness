package domain_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

const nbsp = string(rune(0x00a0))

func readValid(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/valid.md")
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestParsePlan(t *testing.T) {
	doc, problems := domain.Parse(readValid(t))

	if len(problems) != 0 {
		t.Fatalf("valid plan has problems: %v", problems)
	}
	if doc.Track != "full" {
		t.Errorf("track = %q", doc.Track)
	}

	if got := []int{doc.Criteria[0].Line, doc.Criteria[1].Line, doc.Criteria[2].Line}; got[0] != 17 || got[1] != 18 || got[2] != 19 || len(doc.Criteria) != 3 {
		t.Errorf("criteria lines = %v", got)
	}
	if c := doc.Criteria[1]; c.Number != 2 || c.Text != "The query returns the pong and records a span." || !strings.HasPrefix(c.Proof, "`go test ./internal/ping/application") {
		t.Errorf("criterion 2 = %+v", c)
	}
	if doc.CriteriaTable.Line != 15 || strings.Join(doc.CriteriaTable.Header, "|") != "#|Criterion|Proof" {
		t.Errorf("criteria table = %+v", doc.CriteriaTable)
	}

	if len(doc.Tasks) != 2 || doc.Tasks[0].Line != 23 || doc.Tasks[1].Line != 31 {
		t.Fatalf("tasks = %+v", doc.Tasks)
	}
	first, second := doc.Tasks[0], doc.Tasks[1]
	if first.Number != 1 || first.Name != "Ping domain" || first.Side != "backend" || len(first.DependsOn) != 0 {
		t.Errorf("task 1 = %+v", first)
	}
	if strings.Join(first.Owned, ",") != "internal/ping/domain/pong.go,internal/ping/domain/pong_test.go" {
		t.Errorf("task 1 owned = %v", first.Owned)
	}
	if got := second.DoD; len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Errorf("task 2 DoD = %v", got)
	}
	if got := second.DependsOn; len(got) != 1 || got[0] != 1 {
		t.Errorf("task 2 depends = %v", got)
	}
	if !slices.Equal(first.Seams, []string{"domain.Pong"}) || !slices.Equal(second.Seams, []string{"pingquery.Handler"}) {
		t.Errorf("seams = %v %v", first.Seams, second.Seams)
	}
	if got := first.FieldLines[domain.FieldDoD]; got != 28 {
		t.Errorf("task 1 DoD line = %d", got)
	}
	if first.DoD[0] != 1 || len(first.DoD) != 1 || !strings.HasPrefix(second.TestFirst, "`handler_test.go`") {
		t.Errorf("task fields = %+v %+v", first, second)
	}

	if doc.CodeTree.Line != 86 || len(doc.CodeTree.Entries) != 8 {
		t.Fatalf("code tree = %+v", doc.CodeTree)
	}
	handler := doc.CodeTree.Entries[7]
	want := domain.TreeEntry{Path: "internal/ping/application/query/pingquery/handler.go", Mark: domain.MarkNew, Line: 95, Depth: 6}
	if handler != want {
		t.Errorf("handler entry = %+v", handler)
	}
	if dir := doc.CodeTree.Entries[4]; dir.Path != "internal/ping/application/" || !dir.IsDir || dir.Depth != 3 {
		t.Errorf("dir entry = %+v", dir)
	}
	if doc.TestTree.Line != 100 || len(doc.TestTree.Entries) != 8 || doc.TestTree.Entries[3].Path != "internal/ping/domain/pong_test.go" || doc.TestTree.Entries[3].Line != 105 {
		t.Errorf("test tree = %+v", doc.TestTree)
	}
	if !doc.CodeTree.Touches(domain.FolderQuery) || !doc.CodeTree.Touches(domain.FolderDomain) {
		t.Error("code tree must touch query and domain")
	}
	if doc.CodeTree.Touches(domain.FolderCommand, domain.FolderHTTP, domain.FolderConsumer) {
		t.Error("code tree must not touch command, http or consumer")
	}

	if len(doc.Commits) != 3 || doc.Commits[2].Line != 116 || doc.Commits[2].Subject != "docs: describe the ping flow" {
		t.Fatalf("commits = %+v", doc.Commits)
	}
	if got := doc.Commits[2].Tasks; len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("commit 3 tasks = %v", got)
	}

	if len(doc.PRs) != 2 {
		t.Fatalf("prs = %+v", doc.PRs)
	}
	if pr := doc.PRs[0]; pr != (domain.PR{Number: 1, Branch: "feat/ping-query", ThisRun: true, Base: "main", Line: 120}) {
		t.Errorf("pr 1 = %+v", pr)
	}
	if pr := doc.PRs[1]; pr != (domain.PR{Number: 2, Branch: "docs/ping-flow", Line: 121}) {
		t.Errorf("pr 2 = %+v", pr)
	}

	if len(doc.Signals) != 1 {
		t.Fatalf("signals = %+v", doc.Signals)
	}
	wantSignal := domain.SignalRow{Criterion: 2, Signal: "ping calls", Source: "prometheus", Query: "ping:calls:rate5m", Breach: "> 100", Window: "24h", Panel: "ping calls", Line: 43}
	if doc.Signals[0] != wantSignal {
		t.Errorf("signal = %+v", doc.Signals[0])
	}
	if doc.SignalsTable.Line != 41 || len(doc.SignalsTable.Header) != 7 {
		t.Errorf("signals table = %+v", doc.SignalsTable)
	}

	if len(doc.Specs) != 2 {
		t.Fatalf("specs = %+v", doc.Specs)
	}
	if s := doc.Specs[1]; s.Title != "Ping query" || s.Line != 70 || !s.HasCriteriaLine || len(s.Criteria) != 2 || s.Criteria[0] != 2 || s.Criteria[1] != 3 {
		t.Errorf("spec 2 = %+v", s)
	}
	if s := doc.Specs[0]; s.Title != "Ping domain" || s.Line != 52 || len(s.Criteria) != 1 || s.Criteria[0] != 1 {
		t.Errorf("spec 1 = %+v", s)
	}

	if len(doc.Diagrams) != 2 {
		t.Fatalf("diagrams = %+v", doc.Diagrams)
	}
	if d := doc.Diagrams[0]; d.Kind != domain.DiagramClass || d.Line != 59 || !strings.Contains(d.Body, "Handler --> Pong") {
		t.Errorf("diagram 1 = %+v", d)
	}
	if d := doc.Diagrams[1]; d.Kind != domain.DiagramState || d.Line != 77 || strings.Contains(d.Body, "mermaid") {
		t.Errorf("diagram 2 = %+v", d)
	}

	headings := map[string]int{}
	for _, h := range doc.Headings {
		headings[h.Text] = h.Line
	}
	for text, line := range map[string]int{
		domain.HeadingGoal: 3, domain.HeadingCriteria: 13, domain.HeadingTasks: 21, domain.HeadingSignals: 39,
		domain.HeadingDecisions: 45, domain.HeadingSpecs: 50, domain.HeadingCodeTree: 84,
		domain.HeadingTestTree: 98, domain.HeadingCommits: 112, domain.HeadingPRs: 118,
	} {
		if headings[text] != line {
			t.Errorf("heading %q at %d, want %d", text, headings[text], line)
		}
	}
	if len(doc.Lines) < 121 {
		t.Errorf("raw lines = %d", len(doc.Lines))
	}
}

func TestParseSketches(t *testing.T) {
	text := strings.Join([]string{
		"## Tech specs",
		"",
		"### Screen",
		"",
		"The list: ![List](ui/list.excalidraw) and ![Detail](ui/detail.excalidraw).",
		"![Photo](ui/photo.png)",
		"The parser reads `![title](path.excalidraw)` links.",
		"",
		"```",
		"![Fenced](ui/fenced.excalidraw)",
		"```",
		"![Dot](./ui/dot.excalidraw)",
	}, "\n")

	doc, _ := domain.Parse(text)

	want := []domain.Sketch{
		{Title: "List", Path: "ui/list.excalidraw", Line: 5},
		{Title: "Detail", Path: "ui/detail.excalidraw", Line: 5},
		{Title: "Dot", Path: "ui/dot.excalidraw", Line: 12},
	}
	if !slices.Equal(doc.Sketches, want) {
		t.Fatalf("sketches = %+v, want %+v", doc.Sketches, want)
	}
}

func TestParseTaskModel(t *testing.T) {
	text := strings.Join([]string{
		"## Tasks",
		"",
		"### 1. Ping domain",
		"",
		"- Side: backend",
		"- Model: haiku",
		"",
		"### 2. Ping query",
		"",
		"- Side: backend",
	}, "\n")

	doc, problems := domain.Parse(text)

	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	if len(doc.Tasks) != 2 {
		t.Fatalf("tasks = %+v", doc.Tasks)
	}
	if got := doc.Tasks[0].Model; got != "haiku" {
		t.Errorf("task 1 model = %q, want haiku", got)
	}
	if got := doc.Tasks[0].FieldLines[domain.FieldModel]; got != 6 {
		t.Errorf("task 1 model line = %d, want 6", got)
	}
	if got := doc.Tasks[1].Model; got != "" {
		t.Errorf("task 2 model = %q, want empty", got)
	}
}

func TestDocumentBase(t *testing.T) {
	doc, _ := domain.Parse(readValid(t))
	if got := doc.Base(); got != "main" {
		t.Errorf("Base = %q, want main", got)
	}
	doc.PRs = []domain.PR{{Branch: "feat/a", Base: "main"}}
	if got := doc.Base(); got != "" {
		t.Errorf("Base with no run PR = %q, want empty", got)
	}
}

func TestParseProblems(t *testing.T) {
	valid := readValid(t)
	cases := []struct {
		name    string
		from    string
		to      string
		line    int
		rule    domain.RuleID
		message string
	}{
		{"criterion number", "| 2 | The query", "| two | The query", 18, domain.RuleCriteriaTable, "not a number"},
		{"criterion cells", "| 3 | The module builds and passes all tests. |", "| 3 | The module builds and passes all tests. | `make check` | extra |", 19, domain.RuleCriteriaTable, "cells"},
		{"task heading", "### 2. Ping query", "### Ping query", 31, domain.RuleTaskField, "n. Name"},
		{"dod form", "- DoD: criteria 2 to 3.", "- DoD: the second criterion.", 36, domain.RuleTaskField, "DoD"},
		{"depends form", "- Depends on: task 1", "- Depends on: after one", 37, domain.RuleTaskField, "Depends on"},
		{"signal cells", "| `> 100` | `24h` | ping calls |", "| `24h` | ping calls |", 43, domain.RuleSignalsTable, "cells"},
		{"spec criteria", "Criteria: 2 and 3", "Criteria: all of them", 73, domain.RuleSpecCriteria, "Criteria"},
		{"tree indent", "        │   └── pong.go ", "      │   └── pong.go ", 91, domain.RuleTreeFormat, "indent"},
		{"tree mark", "pong.go                    new", "pong.go", 91, domain.RuleTreeFormat, "mark"},
		{"tree name blank", "pong.go                    new", nbsp, 91, domain.RuleTreeFormat, "name"},
		{"dod range too wide", "- DoD: criteria 2 to 3.", "- DoD: criteria 2 to 5000.", 36, domain.RuleTaskField, "DoD"},
		{"commit form", "1. `feat: add the pong domain` (task 1)", "1. feat: add the pong domain (task 1)", 114, domain.RuleCommitFormat, "commit"},
		{"pr form", "(this run), based on `main`:", "(this run), based on main:", 120, domain.RulePRSequence, "PR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if !strings.Contains(valid, tc.from) {
				t.Fatalf("fixture lacks %q", tc.from)
			}
			_, problems := domain.Parse(strings.Replace(valid, tc.from, tc.to, 1))
			if len(problems) != 1 {
				t.Fatalf("problems = %v", problems)
			}
			p := problems[0]
			if p.Line != tc.line || p.Rule != tc.rule || !strings.Contains(p.Message, tc.message) {
				t.Errorf("problem = %v, want line %d rule %s containing %q", p, tc.line, tc.rule, tc.message)
			}
		})
	}
}

func TestParseDiagramKind(t *testing.T) {
	cases := []struct {
		name  string
		block string
		kind  string
		body  string
	}{
		{"comment first", "%% the flow\nstateDiagram-v2\n    a --> b", domain.DiagramState, "%% the flow\nstateDiagram-v2\n    a --> b"},
		{"init directive first", "%%{init: {\"theme\": \"dark\"}}%%\nclassDiagram\n    A --> B", domain.DiagramClass, "%%{init: {\"theme\": \"dark\"}}%%\nclassDiagram\n    A --> B"},
		{"front matter first keeps the line count", "---\ntitle: the flow\n---\nstateDiagram-v2\n    a --> b", domain.DiagramState, "\n\n\nstateDiagram-v2\n    a --> b"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc, _ := domain.Parse("```mermaid\n" + tc.block + "\n```\n")

			if len(doc.Diagrams) != 1 || doc.Diagrams[0].Kind != tc.kind || doc.Diagrams[0].Body != tc.body {
				t.Fatalf("diagrams = %+v, want kind %q and body %q", doc.Diagrams, tc.kind, tc.body)
			}
		})
	}
}

func TestProblemString(t *testing.T) {
	p := domain.Problem{Line: 12, Rule: domain.RuleCriteriaTable, Message: "text"}
	if got := p.String(); got != "plan.md:12: criteria-table: text" {
		t.Errorf("String = %q", got)
	}
}
