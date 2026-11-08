package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/plan/application/query/markdiagram"
)

const commandDiagram = "diagram"

func NewDiagramCommand(handler markdiagram.Handler) *cobra.Command {
	return &cobra.Command{
		Use:   commandDiagram,
		Short: "Mark the diagram on stdin and print it in a mermaid fence",
		Args:  cobra.NoArgs,
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
