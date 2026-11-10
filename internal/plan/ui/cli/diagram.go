package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/plan/application/query/markdiagram"
)

const commandDiagram = "diagram"

const DiagramExample = "flowchart TD\n    api[API] --> store[Store] +\n    worker[Worker] --> api *\n    legacy[Legacy] -\n"

const diagramHelp = `Read a mermaid diagram on stdin. Print it in a mermaid fence with the marks drawn.
Paste the output in the plan. A raw mark in the plan fails "harness plan check".

A mark is a space, then + (new), * (changed) or - (removed), at the end of a line.
Mark at least one line. A node, state or class takes one mark.
These marks are not the tree marks of the plan (new, changed, deleted).

flowchart, graph: node lines and link lines with one link. No chain edge (two links),
  no subgraph line, no "end". A node id is never a mermaid keyword, such as style or end.
stateDiagram, stateDiagram-v2: state lines and transition lines.
classDiagram, classDiagram-v2: "class Name +", "class Name { +" and relation lines.
erDiagram: relation lines and attribute lines inside { }. No entity line, no "}".
Every other kind, such as sequenceDiagram, takes no mark: paste it in a plain mermaid fence.
A refusal names the line and exits 2.`

func NewDiagramCommand(handler markdiagram.Handler) *cobra.Command {
	return &cobra.Command{
		Use:     commandDiagram,
		Short:   "Mark the diagram on stdin and print it in a mermaid fence",
		Long:    diagramHelp,
		Example: "  harness diagram <<'EOF'\n" + DiagramExample + "EOF",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			input, err := io.ReadAll(cmd.InOrStdin())
			if err != nil {
				return err
			}
			body, err := handler.Handle(cmd.Context(), string(input))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "```mermaid\n%s\n```\n", body)
			return nil
		},
	}
}
