package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

const (
	commandGates    = "gates"
	commandInit     = "init"
	gateInitFormat  = "%s: %s\n"
	setupInitFormat = "setup %s: %s\n"
)

func NewGatesCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{Use: commandGates, Short: "Manage the gates"}
	cmd.AddCommand(&cobra.Command{
		Use:   commandInit,
		Short: "Write the detected gates to the config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				written, err := h.Init(ctx)
				if err != nil {
					return err
				}
				for _, step := range written.Setup {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), setupInitFormat, step.ID, step.Run); err != nil {
						return err
					}
				}
				for _, gate := range written.Gates {
					if _, err := fmt.Fprintf(cmd.OutOrStdout(), gateInitFormat, gate.ID, gate.Run); err != nil {
						return err
					}
				}
				return nil
			})
		},
	})
	return cmd
}
