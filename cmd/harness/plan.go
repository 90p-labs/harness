package main

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	planmodule "github.com/jorge07/harness/internal/plan"
	"github.com/jorge07/harness/internal/plan/application/query/checkplan"
	plancli "github.com/jorge07/harness/internal/plan/ui/cli"
	"github.com/jorge07/harness/internal/run/application/query/status"
	"github.com/jorge07/harness/internal/run/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

func newPlanCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := plancli.NewCommand(func(ctx context.Context) (plancli.Handlers, func() error, error) {
		env, err := newEnvironment(ctx, flags)
		if err != nil {
			return plancli.Handlers{}, nil, err
		}
		handlers := planmodule.NewModule(env.bus, env.key.Worktree)
		handlers.DefaultPlan = func(ctx context.Context) (checkplan.Query, error) {
			return activePlan(ctx, flags)
		}
		return handlers, func() error { return nil }, nil
	})
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

func newDiagramCommand() *cobra.Command {
	cmd := plancli.NewDiagramCommand(planmodule.NewDiagram())
	runDiagram := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		err := runDiagram(cmd, args)
		if errors.Is(err, shareddomain.ErrDiagramRefused) {
			return usageError{err}
		}
		return err
	}
	return cmd
}

func planPath(worktree, runID string) string {
	return filepath.Join(worktree, filepath.FromSlash(shareddomain.RunPlanFile(runID)))
}

func activePlan(ctx context.Context, flags *pflag.FlagSet) (query checkplan.Query, err error) {
	handlers, closeStore, err := connect(ctx, flags)
	if err != nil {
		return checkplan.Query{}, err
	}
	defer func() { err = errors.Join(err, closeStore()) }()
	current, err := handlers.Status.Handle(ctx, status.Query{Key: handlers.Key})
	if errors.Is(err, domain.ErrRunNotFound) {
		return checkplan.Query{}, usageError{plancli.ErrNoPlanFile}
	}
	if err != nil {
		return checkplan.Query{}, err
	}
	return checkplan.Query{Path: planPath(handlers.Key.Worktree, current.ID), LiveRound: current.LiveRound}, nil
}
