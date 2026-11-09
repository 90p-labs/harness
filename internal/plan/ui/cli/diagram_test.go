package cli_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/90p-labs/harness/internal/plan/application/query/markdiagram"
	"github.com/90p-labs/harness/internal/plan/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	markedInput   = "flowchart TD\n    a[start] --> b[end] +\n"
	unmarkedInput = "flowchart TD\n    a --> b\n"
	fencedOutput  = "```mermaid\nflowchart TD\n    classDef new stroke:#00b34e,stroke-width:3px,color:#00b34e\n    a[start] -->|\"(new)\"| b[end]\n```\n"
)

func executeDiagram(t *testing.T, input string) (string, error) {
	t.Helper()
	handler := markdiagram.NewHandler(nil, noop.NewTracerProvider().Tracer("test"))
	cmd := cli.NewDiagramCommand(handler)
	var out bytes.Buffer
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetIn(strings.NewReader(input))
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	err := cmd.ExecuteContext(t.Context())
	return out.String(), err
}

func TestDiagramHelp(t *testing.T) {
	t.Run("the example marks with no refusal", func(t *testing.T) {
		out, err := executeDiagram(t, cli.DiagramExample)

		if err != nil {
			t.Fatalf("diagram: %v", err)
		}
		if !strings.HasPrefix(out, "```mermaid\n") {
			t.Errorf("stdout = %q, want a mermaid fence", out)
		}
		for mark, style := range shareddomain.MarkStyles {
			if !strings.Contains(out, style) {
				t.Errorf("stdout = %q, want the %s mark drawn", out, mark)
			}
		}
	})

	t.Run("the printed help names each marked kind and the example", func(t *testing.T) {
		cmd := cli.NewDiagramCommand(markdiagram.NewHandler(nil, noop.NewTracerProvider().Tracer("test")))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"--help"})

		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatalf("help: %v", err)
		}
		for _, kind := range shareddomain.MarkableKindNames {
			if !strings.Contains(out.String(), kind) {
				t.Errorf("help does not name the marked kind %q", kind)
			}
		}
		if !strings.Contains(out.String(), cli.DiagramExample) {
			t.Errorf("help does not hold the example %q", cli.DiagramExample)
		}
		if lines := strings.Count(out.String(), "\n"); lines >= maxHelpLines {
			t.Errorf("help has %d lines, want fewer than %d", lines, maxHelpLines)
		}
	})
}

func TestDiagramCommand(t *testing.T) {
	t.Run("prints the marked block in a fence", func(t *testing.T) {
		out, err := executeDiagram(t, markedInput)

		if err != nil {
			t.Fatalf("diagram: %v", err)
		}
		if out != fencedOutput {
			t.Errorf("stdout = %q, want %q", out, fencedOutput)
		}
	})

	t.Run("refuses an unmarked diagram and prints nothing", func(t *testing.T) {
		out, err := executeDiagram(t, unmarkedInput)

		if !errors.Is(err, shareddomain.ErrDiagramRefused) {
			t.Fatalf("err = %v, want a refused diagram", err)
		}
		if out != "" {
			t.Errorf("stdout = %q, want empty", out)
		}
	})
}
