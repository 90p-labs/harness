package mermaid_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/plan/infrastructure/mermaid"
)

const (
	designPath    = "../../../../DESIGN.md"
	planClassPath = "testdata/plan-class.mmd"
)

var designBlock = regexp.MustCompile("(?s)```mermaid\n(.*?)\n```")

type diagram struct{ kind, body string }

func designDiagrams(t *testing.T) []diagram {
	t.Helper()
	raw, err := os.ReadFile(designPath)
	if err != nil {
		t.Fatal(err)
	}
	var found []diagram
	for _, m := range designBlock.FindAllStringSubmatch(string(raw), -1) {
		kind := strings.Fields(m[1])[0]
		if kind == domain.DiagramState || kind == domain.DiagramClass {
			found = append(found, diagram{kind, m[1]})
		}
	}
	return found
}

func TestParseDiagrams(t *testing.T) {
	parser := mermaid.Parser{}

	t.Run("accepts the diagrams of DESIGN.md", func(t *testing.T) {
		found := designDiagrams(t)
		if len(found) != 4 {
			t.Fatalf("want 4 state and class diagrams in DESIGN.md, got %d", len(found))
		}
		for _, d := range found {
			if err := parser.Parse(d.kind, d.body); err != nil {
				t.Errorf("%s: %v", d.kind, err)
			}
		}
	})

	t.Run("accepts the class diagram of the plan", func(t *testing.T) {
		raw, err := os.ReadFile(planClassPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := parser.Parse(domain.DiagramClass, string(raw)); err != nil {
			t.Error(err)
		}
	})

	accepted := []struct {
		name, kind, body string
	}{
		{"state syntax", domain.DiagramState, `stateDiagram-v2
    direction LR
    %% a comment
    [*] --> idle
    state "Long label" as long
    state group {
        a --> b: go
        [*] --> a
    }
    idle : waiting for work
    idle --> long
    note right of idle
        free text: with colons --> and arrows
    end note
    note left of long : one line
    classDef hot fill:#f00,color:#fff
    class idle,long hot
    long --> [*]`},
		{"state description with an arrow", domain.DiagramState, "stateDiagram-v2\n    idle : run -> review"},
		{"state choice", domain.DiagramState, "stateDiagram-v2\n    state pick <<choice>>\n    idle --> pick"},
		{"state concurrent regions", domain.DiagramState, "stateDiagram-v2\n    state group {\n        a --> b\n        --\n        c --> d\n    }"},
		{"state class shorthand", domain.DiagramState, "stateDiagram-v2\n    classDef new stroke:#0f0\n    idle --> long:::new"},
		{"class syntax", domain.DiagramClass, `classDiagram
    direction LR
    %% a comment
    class Run {
        <<interface>>
        +Field Type
        Method(args) Ret
        +Apply(cmd Command, policy Policy) Run, error
    }
    class Kind {
        <<enumeration>>
        light
    }
    class Empty
    Run --> Kind
    Run ..> Empty : label
    Run ..|> Kind
    Run --|> Kind
    Run *-- Kind
    Run o-- Kind
    Run -- Kind
    Run <|-- Kind
    Run "1" --> "*" Kind : holds
    classDef hot fill:#f00
    cssClass "Run,Kind" hot`},
	}
	for _, c := range accepted {
		t.Run("accepts "+c.name, func(t *testing.T) {
			if err := parser.Parse(c.kind, c.body); err != nil {
				t.Error(err)
			}
		})
	}

	refused := []struct {
		name, kind, body, line, reason string
	}{
		{"state second label colon", domain.DiagramState, "stateDiagram-v2\n    a --> b\n    a --> b: x: y", "line 3", "colon"},
		{"state short edge", domain.DiagramState, "stateDiagram-v2\n    a -> b", "line 2", "edge"},
		{"state long edge", domain.DiagramState, "stateDiagram-v2\n    a ---> b", "line 2", "edge"},
		{"state edge without target", domain.DiagramState, "stateDiagram-v2\n    a --> ", "line 2", "edge"},
		{"state root id", domain.DiagramState, "stateDiagram-v2\n    a --> root", "line 2", "root"},
		{"state root target after a class shorthand", domain.DiagramState, "stateDiagram-v2\n    Idle:::hot --> root", "line 2", "root"},
		{"state second label colon after a class shorthand", domain.DiagramState, "stateDiagram-v2\n    Idle:::hot --> B : x : y", "line 2", "colon"},
		{"state root declaration", domain.DiagramState, "stateDiagram-v2\n    state root {\n    }", "line 2", "root"},
		{"state unclosed block", domain.DiagramState, "stateDiagram-v2\n    state a {\n    b --> c", "line 3", "unclosed"},
		{"class second label colon", domain.DiagramClass, "classDiagram\n    A ..> B : x : y", "line 2", "colon"},
		{"class bad relation token", domain.DiagramClass, "classDiagram\n    A -> B", "line 2", "relation"},
		{"class bad arrow token", domain.DiagramClass, "classDiagram\n    A ==> B", "line 2", "relation"},
		{"class root", domain.DiagramClass, "classDiagram\n    class root", "line 2", "root"},
		{"class root in relation", domain.DiagramClass, "classDiagram\n    A --> root", "line 2", "root"},
		{"class unclosed body", domain.DiagramClass, "classDiagram\n    class A {\n        +X int", "line 3", "unclosed"},
		{"wrong header", domain.DiagramClass, "stateDiagram-v2\n    a --> b", "line 1", "header"},
		{"unknown kind", "erDiagram", "erDiagram\n    a ||--o{ b : c", "", "kind"},
	}
	for _, c := range refused {
		t.Run("refuses "+c.name, func(t *testing.T) {
			err := parser.Parse(c.kind, c.body)
			if err == nil {
				t.Fatal("want an error")
			}
			if !strings.Contains(err.Error(), c.line) || !strings.Contains(err.Error(), c.reason) {
				t.Errorf("want %q and %q in %q", c.line, c.reason, err)
			}
		})
	}
}
