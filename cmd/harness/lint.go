package main

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jorge07/harness/internal/deploy"
	lintmodule "github.com/jorge07/harness/internal/lint"
	"github.com/jorge07/harness/internal/lint/application/command/baseline"
	"github.com/jorge07/harness/internal/lint/application/command/initlint"
	lintcommand "github.com/jorge07/harness/internal/lint/application/command/lint"
	"github.com/jorge07/harness/internal/lint/infrastructure/rulecatalogue"
	lintcli "github.com/jorge07/harness/internal/lint/ui/cli"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

func newLintCommand(flags *pflag.FlagSet, version string) *cobra.Command {
	return lintcli.NewLintCommand(func(ctx context.Context) (lintcli.Handlers, error) {
		env, err := newEnvironment(ctx, flags)
		if err != nil {
			return lintcli.Handlers{}, err
		}
		cfg, worktree := env.cfg, env.key.Worktree
		handlers, err := lintmodule.New(cfg.Lint, cfg.Plugins, worktree, version, deploy.RenderChart(env.bus))
		if err != nil {
			return lintcli.Handlers{}, err
		}
		return lintcli.Handlers{
			Lint: func(ctx context.Context) (shareddomain.Report, error) {
				return handlers.Lint.Handle(ctx, lintcommand.Command{Dir: worktree})
			},
			Init: func(ctx context.Context) error {
				return handlers.Init.Handle(ctx, initlint.Command{Dir: worktree})
			},
			Baseline: func(ctx context.Context) error {
				return handlers.Baseline.Handle(ctx, baseline.WriteBaseline{Dir: worktree, Path: lintmodule.BaselinePath})
			},
			NoPacks: len(cfg.Lint.Packs) == 0,
		}, nil
	}, rulecatalogue.Load)
}
