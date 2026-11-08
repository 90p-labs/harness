package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/plan/application/command/renderview"
)

const (
	commandView = "view"
	flagOut     = "out"
)

func viewCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandView,
		Short: "Write the plan as one HTML page",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			handlers, closeStore, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, closeStore()) }()
			command, err := viewFor(cmd, handlers)
			if err != nil {
				return err
			}
			page, err := handlers.View.Handle(cmd.Context(), command)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), page)
			return nil
		},
	}
	cmd.Flags().String(flagFile, "", "the plan file, default the plan of the active run")
	cmd.Flags().String(flagOut, "", "the page file, default plan.html beside the plan")
	return cmd
}

func viewFor(cmd *cobra.Command, handlers Handlers) (renderview.Command, error) {
	command := renderview.Command{}
	if path, _ := cmd.Flags().GetString(flagFile); path != "" {
		command.Path = path
	} else {
		var err error
		if command, err = handlers.DefaultView(cmd.Context()); err != nil {
			return renderview.Command{}, err
		}
	}
	if out, _ := cmd.Flags().GetString(flagOut); out != "" {
		command.Out = out
	}
	return command, nil
}
