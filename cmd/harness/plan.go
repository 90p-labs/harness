package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/90p-labs/harness/internal/host/infrastructure/install"
	planmodule "github.com/90p-labs/harness/internal/plan"
	"github.com/90p-labs/harness/internal/plan/application/command/renderview"
	"github.com/90p-labs/harness/internal/plan/application/query/checkplan"
	plancli "github.com/90p-labs/harness/internal/plan/ui/cli"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
)

func newPlanCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := plancli.NewCommand(func(ctx context.Context) (plancli.Handlers, func() error, error) {
		env, err := newEnvironment(ctx, flags)
		if err != nil {
			return plancli.Handlers{}, nil, err
		}
		handlers := planmodule.NewModule(env.bus, env.key.Worktree, env.proofs, env.defaultBranch)
		handlers.RepoTop = env.key.Worktree
		handlers.AdvisorModels = advisorModels(env.cfg.Roles, install.DetectHosts(env.key.Worktree, os.Getenv))
		handlers.DefaultPlan = func(ctx context.Context) (checkplan.Query, error) {
			return activePlan(ctx, flags, env.key.Worktree)
		}
		handlers.DefaultView = func(ctx context.Context) (renderview.Command, error) {
			return activeView(ctx, flags, env.key.Worktree)
		}
		return handlers, func() error { return nil }, nil
	})
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

// advisorModels gives the advisor models of the one detected host, and of claude when no host or many are detected.
func advisorModels(roles map[string]config.RoleHost, detected []shareddomain.Host) []string {
	host := shareddomain.HostClaude
	if hosts := hostsOrKnown(detected); len(hosts) == 1 {
		host = hosts[0]
	}
	return roles[string(shareddomain.RoleAdvisor)][string(host)]
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

func activePlan(ctx context.Context, flags *pflag.FlagSet, dir string) (checkplan.Query, error) {
	current, worktree, err := activeRun(ctx, flags, dir)
	if err != nil {
		return checkplan.Query{}, err
	}
	return checkplan.Query{Path: planPath(worktree, current.ID), LiveRound: current.LiveRound}, nil
}

func activeView(ctx context.Context, flags *pflag.FlagSet, dir string) (renderview.Command, error) {
	current, worktree, err := activeRun(ctx, flags, dir)
	if err != nil {
		return renderview.Command{}, err
	}
	return renderview.Command{
		Path: planPath(worktree, current.ID), LiveRound: current.LiveRound,
		RunID: current.ID, Phase: string(current.Phase), Proofs: true,
	}, nil
}

// activeRun reads the active run of the repo that holds dir.
func activeRun(ctx context.Context, flags *pflag.FlagSet, dir string) (current status.Status, worktree string, err error) {
	env, err := environmentAt(ctx, flags, dir)
	if err != nil {
		return status.Status{}, "", err
	}
	defer func() { err = errors.Join(err, env.state.Close()) }()
	handlers, _, err := openRun(ctx, env)
	if err != nil {
		return status.Status{}, "", err
	}
	current, err = handlers.Status.Handle(ctx, status.Query{Key: handlers.Key})
	if errors.Is(err, domain.ErrRunNotFound) {
		return status.Status{}, "", usageError{plancli.ErrNoPlanFile}
	}
	return current, handlers.Key.Worktree, err
}
