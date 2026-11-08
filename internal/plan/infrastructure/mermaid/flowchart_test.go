package mermaid_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/plan/infrastructure/mermaid"
)

const (
	markedStatePath   = "testdata/marked-state.mmd"
	markedStateGolden = "testdata/marked-state.golden"
	markedStateHeader = "stateDiagram-v2\n    classDef new stroke:#00b34e,stroke-width:3px,color:#00b34e\n"
)

func TestStateChart(t *testing.T) {
	input, err := os.ReadFile(markedStatePath)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(markedStateGolden)
	if err != nil {
		t.Fatal(err)
	}

	got, err := mermaid.StateChart(string(input))
	if err != nil {
		t.Fatal(err)
	}

	if got != strings.TrimSuffix(string(want), "\n") {
		t.Errorf("flowchart differs from the golden:\n%s", got)
	}
}

func TestStateChartKeepsMarks(t *testing.T) {
	t.Run("class lines stay", func(t *testing.T) {
		got, err := mermaid.StateChart(markedStateHeader + "    [*] --> a\n    class a new\n")
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range []string{"flowchart TB", "classDef new stroke:#00b34e", "class a new"} {
			if !strings.Contains(got, line) {
				t.Errorf("missing %q in:\n%s", line, got)
			}
		}
	})

	unchanged := map[string]string{
		"unmarked state":  "stateDiagram-v2\n    [*] --> a\n    a --> b: go\n",
		"classDiagram":    "classDiagram\n    class A\n    class A new\n",
		"erDiagram":       "erDiagram\n    A ||--o{ B : has \"(new)\"\n",
		"sequenceDiagram": "sequenceDiagram\n    A->>B: hi (new)\n",
	}
	for name, body := range unchanged {
		t.Run(name, func(t *testing.T) {
			got, err := mermaid.StateChart(body)
			if err != nil || got != body {
				t.Errorf("want body unchanged, got %q, %v", got, err)
			}
		})
	}

	refused := map[string]string{
		"note":   "    note right of a : hi\n",
		"fork":   "    state f <<fork>>\n",
		"join":   "    state j <<join>>\n",
		"choice": "    state c <<choice>>\n",
		"region": "    state r {\n        [*] --> x\n        --\n        [*] --> y\n    }\n",
		"close":  "    }\n    a --> [*]\n",
	}
	for name, line := range refused {
		t.Run(name, func(t *testing.T) {
			body := markedStateHeader + "    [*] --> a\n    class a new\n" + line
			_, err := mermaid.StateChart(body)
			if !errors.Is(err, mermaid.ErrStateChart) {
				t.Fatalf("want ErrStateChart, got %v", err)
			}
		})
	}

	t.Run("error names the line", func(t *testing.T) {
		_, err := mermaid.StateChart(markedStateHeader + "    [*] --> a\n    class a new\n    note right of a : hi\n")
		if err == nil || !strings.Contains(err.Error(), "line 5") {
			t.Errorf("want line 5, got %v", err)
		}
	})
}
