package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/guard"
	livemodule "github.com/jorge07/harness/internal/live"
	livecli "github.com/jorge07/harness/internal/live/ui/cli"
	planmodule "github.com/jorge07/harness/internal/plan"
	"github.com/jorge07/harness/internal/plan/application/command/submitplan"
	runmodule "github.com/jorge07/harness/internal/run"
	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/run/ui/cli"
	"github.com/jorge07/harness/internal/shared/application/eventbus"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/dbal"
	"github.com/jorge07/harness/internal/shared/infrastructure/execplugin"
	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

func newRunCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := cli.NewCommand(func(ctx context.Context) (cli.Handlers, func() error, error) {
		return connect(ctx, flags)
	})
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

type environment struct {
	key       domain.RunKey
	commonDir string
	cfg       config.Config
	bus       eventbus.Bus

	proofs        proofAdapter
	defaultBranch planmodule.DefaultBranch
}

func newEnvironment(ctx context.Context, flags *pflag.FlagSet) (environment, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return environment{}, err
	}
	repo, worktree, err := git.RepoKey(ctx, cwd)
	if err != nil {
		return environment{}, err
	}
	commonDir, err := git.CommonDir(ctx, cwd)
	if err != nil {
		return environment{}, err
	}
	cfg, err := config.Load(flags, worktree)
	if err != nil {
		return environment{}, err
	}
	plugins, err := execplugin.FromConfig(cfg.Plugins)
	if err != nil {
		return environment{}, err
	}
	key := domain.RunKey{Repo: repo, Worktree: worktree}
	guards, err := guard.NewPlugins(cfg.Guard.Disabled, cfg.Git.DefaultBranch, modelsByHost(cfg.Roles), worktree,
		stopAdapter{cfg: cfg, commonDir: commonDir, key: key},
		commitCheckAdapter{cfg: cfg}.Passed)
	if err != nil {
		return environment{}, err
	}
	proofs := proofAdapter{flags: flags, cfg: cfg, commonDir: commonDir, key: key}
	defaultBranch := func(ctx context.Context) (string, error) {
		if cfg.Git.DefaultBranch != "" {
			return cfg.Git.DefaultBranch, nil
		}
		return git.DefaultBranch(ctx, worktree)
	}
	review := planmodule.NewReviewPlugin(worktree, proofs, defaultBranch)
	planView := planmodule.NewViewPlugin(worktree, proofs, defaultBranch, func(ctx context.Context) (int, error) {
		current, _, err := activeRun(ctx, flags)
		return current.LiveRound, err
	})
	var bus eventbus.Bus
	session := livemodule.NewSessionPlugin(func(ctx context.Context) ([]string, error) {
		results, err := runLiveCheck(ctx, environment{key: key, commonDir: commonDir, cfg: cfg, bus: bus})
		return livecli.Lines(results), err
	})
	bus = eventbus.NewBus(append(append(plugins, guards...), review, planView, session), otel.GetTracerProvider(), otel.GetMeterProvider())
	return environment{key: key, commonDir: commonDir, cfg: cfg, bus: bus, proofs: proofs, defaultBranch: defaultBranch}, nil
}

func connect(ctx context.Context, flags *pflag.FlagSet) (cli.Handlers, func() error, error) {
	env, err := newEnvironment(ctx, flags)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	handlers, db, err := openRun(ctx, env)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	return handlers, db.Close, nil
}

func openRun(ctx context.Context, env environment) (cli.Handlers, *dbal.DB, error) {
	db, err := openState(ctx, env.cfg, env.commonDir)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	gate := planGate{submit: planmodule.NewModule(env.bus, env.key.Worktree, env.proofs, env.defaultBranch).Submit, worktree: env.key.Worktree}
	return runmodule.NewModule(db, env.bus, gate, env.cfg.Run, env.key, time.Now), db, nil
}

type planGate struct {
	submit   submitplan.Handler
	worktree string
}

func (g planGate) Submit(ctx context.Context, run domain.Run) (shareddomain.Track, error) {
	return g.submit.Handle(ctx, submitplan.Command{
		RunID: run.ID, Phase: string(run.Phase),
		Path: planPath(g.worktree, run.ID), LiveRound: run.LiveRound,
	})
}

func wrapArgs(cmd *cobra.Command) {
	if cmd.Args == nil {
		cmd.Args = cobra.NoArgs
	}
	cmd.Args = usageArgs(cmd.Args)
	for _, child := range cmd.Commands() {
		wrapArgs(child)
	}
}
