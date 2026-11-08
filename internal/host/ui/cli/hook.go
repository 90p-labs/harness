package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/host/application/command/handlehook"
	hostdomain "github.com/jorge07/harness/internal/host/domain"
)

const commandHook = "hook"

func NewHookCommand(handler handlehook.Handler) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandHook + " HOOK",
		Short: "Handle one hook call of an agent host",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host, err := hostFlag(cmd)
			if err != nil {
				return err
			}
			hook, err := hostdomain.ParseHookName(args[0])
			if err != nil {
				return err
			}
			out := handler.Handle(cmd.Context(), handlehook.Command{Host: host, Hook: hook, Input: cmd.InOrStdin()})
			if _, err := cmd.OutOrStdout().Write(out.Stdout); err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.ErrOrStderr(), out.Stderr)
			return err
		},
	}
	addHostFlag(cmd)
	return cmd
}
