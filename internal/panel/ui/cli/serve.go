// Package cli holds the panel command.
package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const (
	commandPanel = "panel"
	commandServe = "serve"
	flagPort     = "port"
)

// Serve runs the panel until the context ends. It calls ready with the URL once it listens.
type Serve func(ctx context.Context, port int, ready func(url string)) error

// NewCommand builds `harness panel` with its serve, start, stop and status subcommands.
func NewCommand(serve Serve, life Lifecycle) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandPanel,
		Short: "Show the runs of this repo in a local control panel",
	}
	serveCmd := &cobra.Command{
		Use:   commandServe,
		Short: "Serve the board and the run pages on loopback until interrupted",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			port, err := cmd.Flags().GetInt(flagPort)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			var writeErr error
			ready := func(url string) { _, writeErr = fmt.Fprintf(out, "panel: %s\n", url) }
			if err := serve(cmd.Context(), port, ready); err != nil {
				return err
			}
			return writeErr
		},
	}
	serveCmd.Flags().Int(flagPort, 0, "loopback port to listen on, 0 picks a free one")
	cmd.AddCommand(serveCmd, startCommand(life), stopCommand(life), statusCommand(life))
	return cmd
}
