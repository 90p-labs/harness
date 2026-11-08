package domain_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/shared/domain"
)

const (
	defNew     = "classDef new stroke:#00b34e,stroke-width:3px,color:#00b34e"
	defChanged = "classDef changed stroke:#e0a84a,stroke-width:3px,color:#e0a84a"
	defRemoved = "classDef removed stroke:#e2001f,stroke-width:3px,color:#e2001f,stroke-dasharray:5 5"
)

func lines(s ...string) string { return strings.Join(s, "\n") }

func assertMarked(t *testing.T, input, want string) {
	t.Helper()
	got, err := domain.MarkDiagram(input)
	if err != nil {
		t.Fatalf("MarkDiagram error: %v", err)
	}
	if got != want {
		t.Fatalf("MarkDiagram =\n%s\nwant\n%s", got, want)
	}
}

func TestMarkDiagramNodes(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{
			"flowchart",
			lines(
				"%% comment +",
				"flowchart TD",
				"    a[Start] +",
				"    b(Mid) *",
				"    c -",
				`    d["x --> y"]  +`,
				"    %% keep +",
				"    a --> b",
			),
			lines(
				"%% comment +",
				"flowchart TD",
				"    "+defNew,
				"    "+defChanged,
				"    "+defRemoved,
				"    a[Start]",
				"    b(Mid)",
				"    c",
				`    d["x --> y"]`,
				"    %% keep +",
				"    a --> b",
				"    class a new",
				"    class b changed",
				"    class c removed",
				"    class d new",
			),
		},
		{
			"trailing newline and CRLF",
			"flowchart TD\r\n    a\t+\r\n\r\n",
			lines("flowchart TD", "    "+defNew, "    a", "    class a new"),
		},
		{
			"front matter",
			lines("---", "title: t", "---", "flowchart TD", "    a +"),
			lines("---", "title: t", "---", "flowchart TD", "    "+defNew, "    a", "    class a new"),
		},
		{
			"flowchart node named note",
			lines("flowchart TD", "    note +"),
			lines("flowchart TD", "    "+defNew, "    note", "    class note new"),
		},
		{
			"state",
			lines(
				"stateDiagram-v2",
				"    [*] --> Idle",
				"    state Busy +",
				"    Done *",
				"    note right of Done : retry -",
			),
			lines(
				"stateDiagram-v2",
				"    "+defNew,
				"    "+defChanged,
				"    [*] --> Idle",
				"    state Busy",
				"    Done",
				"    note right of Done : retry -",
				"    class Busy new",
				"    class Done changed",
			),
		},
		{
			"class",
			lines(
				"classDiagram-v2",
				"    class Repo { +",
				"        +Save() error",
				"    }",
				"    class Old -",
			),
			lines(
				"classDiagram-v2",
				"    "+defNew,
				"    "+defRemoved,
				"    class Repo {",
				"        +Save() error",
				"    }",
				"    class Old",
				`    cssClass "Repo" new`,
				`    cssClass "Old" removed`,
			),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assertMarked(t, c.input, c.want) })
	}
}

func TestMarkDiagramEdges(t *testing.T) {
	cases := []struct{ name, input, want string }{
		{
			"flowchart",
			lines(
				"flowchart LR",
				"    a --> b +",
				`    b -->|ok| c *`,
				"    c -.-> d -",
				"    d ---> e  +",
				`    e -->|"x"| f +`,
			),
			lines(
				"flowchart LR",
				"    "+defNew,
				"    "+defChanged,
				"    "+defRemoved,
				`    a -->|"(new)"| b`,
				`    b -->|"ok (changed)"| c`,
				`    c -.->|"(removed)"| d`,
				`    d --->|"(new)"| e`,
				`    e -->|"x (new)"| f`,
			),
		},
		{
			"flowchart text labels",
			lines(
				"flowchart LR",
				"    a -- yes --> b +",
				"    b == heavy ==> c *",
				`    c -. "dot" .-> d -`,
			),
			lines(
				"flowchart LR",
				"    "+defNew,
				"    "+defChanged,
				"    "+defRemoved,
				`    a -- "yes (new)" --> b`,
				`    b == "heavy (changed)" ==> c`,
				`    c -. "dot (removed)" .-> d`,
			),
		},
		{
			"state",
			lines(
				"stateDiagram-v2",
				"    [*] --> Idle +",
				"    Idle --> Busy : start *",
				"    Busy --> Idle: retry -",
			),
			lines(
				"stateDiagram-v2",
				"    "+defNew,
				"    "+defChanged,
				"    "+defRemoved,
				"    [*] --> Idle : (new)",
				"    Idle --> Busy : start (changed)",
				"    Busy --> Idle: retry (removed)",
			),
		},
		{
			"class",
			lines(
				"classDiagram",
				"    A <|-- B -",
			),
			lines(
				"classDiagram",
				"    "+defRemoved,
				"    A <|-- B : (removed)",
			),
		},
		{
			"er relations and attributes",
			lines(
				"erDiagram",
				"    CUSTOMER ||--o{ ORDER : places +",
				`    CUSTOMER ||--o{ INVOICE : "bills" *`,
				"    CUSTOMER ||--o{ NOTE -",
				"    CUSTOMER {",
				"        string name +",
				`        string id PK "key" *`,
				"    }",
			),
			lines(
				"erDiagram",
				`    CUSTOMER ||--o{ ORDER : "places (new)"`,
				`    CUSTOMER ||--o{ INVOICE : "bills (changed)"`,
				`    CUSTOMER ||--o{ NOTE : "(removed)"`,
				"    CUSTOMER {",
				`        string name "(new)"`,
				`        string id PK "key (changed)"`,
				"    }",
			),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assertMarked(t, c.input, c.want) })
	}
}

func TestMarkDiagramRefuses(t *testing.T) {
	cases := []struct{ name, input, wantLine string }{
		{"empty input", "", "line 1:"},
		{"unknown kind", lines("foo", "    a +"), "line 1:"},
		{"unmarkable kind", lines("sequenceDiagram", "    A->>B: hi +"), "line 1:"},
		{"no mark", lines("flowchart TD", "    a --> b"), "line 1:"},
		{"unknown mark", lines("flowchart TD", "    a[start] +", "    d[Plain] ?"), "line 3:"},
		{"er entity block", lines("erDiagram", "    CUSTOMER { +", "    }"), "line 2:"},
		{"er bare entity", lines("erDiagram", "    CUSTOMER +"), "line 2:"},
		{"er block end", lines("erDiagram", "    A {", "        string name", "    } +"), "line 4:"},
		{"flowchart block end", lines("flowchart TD", "    subgraph X", "    a --> b", "    end +"), "line 4:"},
		{"no accepted form", lines("flowchart TD", "    a[ok] *", "    subgraph X +"), "line 3:"},
		{"chain edge", lines("flowchart TD", "    a --> b --> c +"), "line 2:"},
		{"two marks on one node", lines("flowchart TD", "    a[x] +", "    a[y] *"), "line 3:"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := domain.MarkDiagram(c.input)
			if !errors.Is(err, domain.ErrDiagramRefused) {
				t.Fatalf("err = %v, want ErrDiagramRefused", err)
			}
			if !strings.Contains(err.Error(), c.wantLine) {
				t.Fatalf("err = %q, want it to hold %q", err, c.wantLine)
			}
			if got != "" {
				t.Fatalf("body = %q, want empty", got)
			}
		})
	}
}

func TestDiagramKind(t *testing.T) {
	cases := []struct{ name, body, want string }{
		{"header after a comment", lines("%% note", "", "flowchart TD", "    a"), "flowchart"},
		{"header after front matter", lines("---", "title: flowchart", "---", "stateDiagram-v2"), "stateDiagram-v2"},
		{"no header", lines("", "%% only"), ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.DiagramKind(c.body); got != c.want {
				t.Fatalf("DiagramKind = %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseMarks(t *testing.T) {
	cases := []struct {
		name string
		body string
		want domain.Marks
	}{
		{
			"state diagram from harness diagram",
			lines("stateDiagram-v2", "    "+defNew, "    "+defChanged, " a --> b: go (new)", " b", "    class b changed"),
			domain.Marks{
				Nodes: map[string]domain.Mark{"b": domain.MarkChanged},
				Edges: map[int]domain.Mark{3: domain.MarkNew},
			},
		},
		{
			"flowchart with a comma list and a label mark",
			lines("flowchart TD", "    "+defNew, "    "+defRemoved, `    a -->|"(removed)"| b`, "    c --> d", "    class a,b new", "    class c removed"),
			domain.Marks{
				Nodes: map[string]domain.Mark{"a": domain.MarkNew, "b": domain.MarkNew, "c": domain.MarkRemoved},
				Edges: map[int]domain.Mark{3: domain.MarkRemoved},
			},
		},
		{
			"class diagram with cssClass lines",
			lines("classDiagram", "    "+defNew, "    "+defChanged, "    class A", "    class B", "    A --> B : (changed)", `    cssClass "A" new`, `    cssClass "B" changed`),
			domain.Marks{
				Nodes: map[string]domain.Mark{"A": domain.MarkNew, "B": domain.MarkChanged},
				Edges: map[int]domain.Mark{5: domain.MarkChanged},
			},
		},
		{
			"a class that is not a mark gives no entry",
			lines("flowchart TD", "    a --> b", "    class a,b highlight"),
			domain.Marks{Nodes: map[string]domain.Mark{}, Edges: map[int]domain.Mark{}},
		},
		{
			"an unmarked kind gives no entry",
			lines("sequenceDiagram", "    A->>B: go (new)"),
			domain.Marks{Nodes: map[string]domain.Mark{}, Edges: map[int]domain.Mark{}},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := domain.ParseMarks(c.body)

			if !maps.Equal(got.Nodes, c.want.Nodes) || !maps.Equal(got.Edges, c.want.Edges) {
				t.Fatalf("ParseMarks = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestCheckMarks(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		{
			"valid flowchart",
			lines("flowchart TD", "    "+defNew, "    a --> b", "    class a new"),
			nil,
		},
		{
			"valid class diagram",
			lines("classDiagram", "    "+defChanged, "    class A {", "    }", `    cssClass "A" changed`),
			nil,
		},
		{
			"label mark needs no classDef",
			lines("flowchart TD", `    a -->|"(new)"| b`),
			nil,
		},
		{
			"state label mark",
			lines("stateDiagram-v2", "    A --> B: go (new)"),
			nil,
		},
		{
			"er label mark",
			lines("erDiagram", `    A ||--o{ B : "x (removed)"`),
			nil,
		},
		{
			"semicolons end the statements",
			lines("flowchart TD", "    "+defNew+";", "    a --> b", "    class a new;"),
			nil,
		},
		{
			"shorthand class use",
			lines("flowchart TD", "    "+defNew, "    a:::new --> b"),
			nil,
		},
		{"no mark", lines("flowchart TD", "    a --> b"), []string{"no mark"}},
		{"er no mark", lines("erDiagram", "    A ||--o{ B : x"), []string{"no mark"}},
		{"class v2 no mark", lines("classDiagram-v2", "    class A"), []string{"no mark"}},
		{"node label is no mark", lines("flowchart TD", "    a[x (new)]"), []string{"no mark"}},
		{"unmarkable kind", lines("sequenceDiagram", "    A->>B: hi"), nil},
		{
			"raw mark left",
			lines("flowchart TD", "    "+defNew, "    class a new", "    b --> c  *"),
			[]string{"line 4 keeps the raw mark *"},
		},
		{
			"class lines in an erDiagram",
			lines("erDiagram", "    "+defNew, "    class A new", `    A ||--o{ B : "x (new)"`),
			[]string{"erDiagram takes no classDef or class line"},
		},
		{
			"another style",
			lines("flowchart TD", "    classDef new fill:red", "    class a new"),
			[]string{"classDef new has another style"},
		},
		{
			"semicolon stays in the style",
			lines("flowchart TD", "    classDef new fill:#fff;stroke:#00b34e", "    class a new"),
			[]string{"classDef new has another style"},
		},
		{
			"another name",
			lines("flowchart TD", "    classDef foo fill:red", "    class a foo", "    b:::bar"),
			[]string{"classDef foo has another name", "class foo is not a mark", "class bar is not a mark"},
		},
		{
			"missing classDef reported once",
			lines("flowchart TD", "    class a new", "    class b new"),
			[]string{"classDef new is missing"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.CheckMarks(c.body); !slices.Equal(got, c.want) {
				t.Fatalf("CheckMarks = %q, want %q", got, c.want)
			}
		})
	}
}
