package main

import (
	"context"
	"errors"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jorge07/harness/internal/host"
	hostdomain "github.com/jorge07/harness/internal/host/domain"
	hostcli "github.com/jorge07/harness/internal/host/ui/cli"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

func newHookCommand(flags *pflag.FlagSet) *cobra.Command {
	handler := host.NewHookHandler(func(ctx context.Context) (shareddomain.Dispatcher, error) {
		env, err := newEnvironment(ctx, flags)
		return runDispatcher{next: env.bus}, err
	})
	cmd := hostcli.NewHookCommand(handler)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return err })
	return cmd
}

func newHostCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := hostcli.NewHostCommand(host.NewHostConnect(func(ctx context.Context) (string, error) {
		env, err := newEnvironment(ctx, flags)
		return env.key.Worktree, err
	}))
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	for _, sub := range cmd.Commands() {
		runSub := sub.RunE
		sub.RunE = func(cmd *cobra.Command, args []string) error {
			return asUsageError(runSub(cmd, args))
		}
	}
	return cmd
}

func asUsageError(err error) error {
	if errors.Is(err, hostcli.ErrUnknownHost) || errors.Is(err, hostdomain.ErrUnknownHook) ||
		errors.Is(err, hostdomain.ErrNoHost) || errors.Is(err, hostdomain.ErrManyHosts) {
		return usageError{err}
	}
	return err
}
