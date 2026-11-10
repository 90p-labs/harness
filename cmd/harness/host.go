package main

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/90p-labs/harness/internal/host"
	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	hostpersistence "github.com/90p-labs/harness/internal/host/infrastructure/persistence"
	hostcli "github.com/90p-labs/harness/internal/host/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func newHookCommand(flags *pflag.FlagSet) *cobra.Command {
	marks := &runMarks{}
	handler := host.NewHookHandler(func(ctx context.Context) (shareddomain.Dispatcher, error) {
		env, err := newEnvironment(ctx, flags)
		marks.state = env.state
		return runDispatcher{next: env.bus}, err
	}, marks)
	cmd := hostcli.NewHookCommand(handler)
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return err })
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		defer func() { _ = marks.state.Close() }()
		return run(cmd, args)
	}
	return cmd
}

type runMarks struct {
	state *stateHolder
}

func (m runMarks) Open(ctx context.Context, kind hostdomain.MarkKind, id string, at time.Time) error {
	runID := runRefFrom(ctx).ID
	if runID == "" {
		return nil
	}
	return m.withMarks(ctx, func(marks hostpersistence.SQLAgentMarks) error {
		return marks.Open(ctx, runID, kind, id, at)
	})
}

func (m runMarks) Close(ctx context.Context, kind hostdomain.MarkKind, id string) (started time.Time, found bool, err error) {
	runID := runRefFrom(ctx).ID
	if runID == "" {
		return time.Time{}, false, nil
	}
	err = m.withMarks(ctx, func(marks hostpersistence.SQLAgentMarks) (closeErr error) {
		started, found, closeErr = marks.Close(ctx, runID, kind, id)
		return closeErr
	})
	return started, found, err
}

func (m runMarks) OpenTool(ctx context.Context, mark hostdomain.ToolMark) error {
	runID := runRefFrom(ctx).ID
	if runID == "" {
		return nil
	}
	return m.withMarks(ctx, func(marks hostpersistence.SQLAgentMarks) error {
		return marks.OpenTool(ctx, runID, mark)
	})
}

func (m runMarks) CloseSession(ctx context.Context, sessionID string) (closed []hostdomain.ToolMark, err error) {
	runID := runRefFrom(ctx).ID
	if runID == "" {
		return nil, nil
	}
	err = m.withMarks(ctx, func(marks hostpersistence.SQLAgentMarks) (closeErr error) {
		closed, closeErr = marks.CloseSession(ctx, runID, sessionID)
		return closeErr
	})
	return closed, err
}

func (m runMarks) withMarks(ctx context.Context, use func(hostpersistence.SQLAgentMarks) error) error {
	db, err := m.state.DB(ctx)
	if err != nil {
		return err
	}
	return use(hostpersistence.NewSQLAgentMarks(db))
}

func newHostCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := hostcli.NewHostCommand(host.NewHostConnect(func(ctx context.Context) (string, bool, error) {
		env, err := newEnvironment(ctx, flags)
		return env.key.Worktree, env.cfg.Hooks.Matched, err
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
