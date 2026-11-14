package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/panel/domain"
)

// noPort tells start to use the configured port. The flag keeps its help default 0.
const noPort = -1

// Lifecycle is what the lifecycle commands call. The module builds it from the handlers.
type Lifecycle struct {
	// Version is the version of this binary. A start that finds a panel of
	// another version running leaves it and says how to switch.
	Version string
	// Prepare runs at the start of the start command, before the version check,
	// so a start that reuses a panel of another version still prepares the repo.
	// It may be nil.
	Prepare func(ctx context.Context) error
	// Start runs the panel in the background. A port below 0 uses the configured port.
	Start func(ctx context.Context, port int) error
	// Stop ends the panel.
	Stop func(ctx context.Context) error
	// Status returns the running panel, or domain.ErrStopped.
	Status func(ctx context.Context) (domain.Instance, error)
}

func startCommand(life Lifecycle) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Run the panel in the background and print its URL",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			port := noPort
			if cmd.Flags().Changed(flagPort) {
				var err error
				if port, err = cmd.Flags().GetInt(flagPort); err != nil {
					return err
				}
			}
			if life.Prepare != nil {
				if err := life.Prepare(cmd.Context()); err != nil {
					return err
				}
			}
			if life.Version != "" {
				if run, err := life.Status(cmd.Context()); err == nil && run.Version != life.Version {
					_, err := fmt.Fprintf(cmd.OutOrStdout(),
						"panel: running version %s, run harness panel stop to switch to %s\npanel: %s\n",
						run.Version, life.Version, run.URL)
					return err
				}
			}
			if err := life.Start(cmd.Context(), port); err != nil {
				return err
			}
			inst, err := life.Status(cmd.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "panel: %s\n", inst.URL)
			return err
		},
	}
	cmd.Flags().Int(flagPort, 0, "loopback port to listen on, 0 picks a free one")
	return cmd
}

func stopCommand(life Lifecycle) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background panel",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := life.Stop(cmd.Context()); err != nil {
				return err
			}
			_, err := fmt.Fprintln(cmd.OutOrStdout(), "panel: stopped")
			return err
		},
	}
}

func statusCommand(life Lifecycle) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Print the URL of the running panel, or fail when it is stopped",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			inst, err := life.Status(cmd.Context())
			if errors.Is(err, domain.ErrStopped) {
				if _, werr := fmt.Fprintln(cmd.OutOrStdout(), "panel: stopped"); werr != nil {
					return werr
				}
				return err
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "panel: %s pid %d\n", inst.URL, inst.PID)
			return err
		},
	}
}
