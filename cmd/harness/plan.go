package main

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	planmodule "github.com/jorge07/harness/internal/plan"
	"github.com/jorge07/harness/internal/plan/application/command/renderview"
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
		handlers := planmodule.NewModule(env.bus, env.key.Worktree, env.proofs, env.defaultBranch)
		handlers.DefaultPlan = func(ctx context.Context) (checkplan.Query, error) {
			return activePlan(ctx, flags)
		}
		handlers.DefaultView = func(ctx context.Context) (renderview.Command, error) {
			return activeView(ctx, flags)
		}
		return handlers, func() error { return nil }, nil
	})
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

const diagramHint = "hint: harness diagram --help"

func newDiagramCommand() *cobra.Command {
	cmd := plancli.NewDiagramCommand(planmodule.NewDiagram())
	runDiagram := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		err := runDiagram(cmd, args)
		if errors.Is(err, shareddomain.ErrDiagramRefused) {
			return refusalError{error: err, hint: diagramHint}
		}
		return err
	}
	return cmd
}

func planPath(worktree, runID string) string {
	return filepath.Join(worktree, filepath.FromSlash(shareddomain.RunPlanFile(runID)))
}

func activePlan(ctx context.Context, flags *pflag.FlagSet) (checkplan.Query, error) {
	current, worktree, err := activeRun(ctx, flags)
	if err != nil {
		return checkplan.Query{}, err
	}
	return checkplan.Query{Path: planPath(worktree, current.ID), LiveRound: current.LiveRound}, nil
}

func activeView(ctx context.Context, flags *pflag.FlagSet) (renderview.Command, error) {
	current, worktree, err := activeRun(ctx, flags)
	if err != nil {
		return renderview.Command{}, err
	}
	return renderview.Command{
		Path: planPath(worktree, current.ID), LiveRound: current.LiveRound,
		RunID: current.ID, Phase: string(current.Phase), Proofs: true,
	}, nil
}

func activeRun(ctx context.Context, flags *pflag.FlagSet) (current status.Status, worktree string, err error) {
	handlers, closeStore, err := connect(ctx, flags)
	if err != nil {
		return status.Status{}, "", err
	}
	defer func() { err = errors.Join(err, closeStore()) }()
	current, err = handlers.Status.Handle(ctx, status.Query{Key: handlers.Key})
	if errors.Is(err, domain.ErrRunNotFound) {
		return status.Status{}, "", usageError{plancli.ErrNoPlanFile}
	}
	return current, handlers.Key.Worktree, err
}
