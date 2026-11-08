package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	planmodule "github.com/jorge07/harness/internal/plan"
	"github.com/jorge07/harness/internal/plan/application/command/submitplan"
	runmodule "github.com/jorge07/harness/internal/run"
	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/run/infrastructure/persistence"
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
	bus := eventbus.NewBus(plugins, otel.GetTracerProvider(), otel.GetMeterProvider())
	return environment{key: domain.RunKey{Repo: repo, Worktree: worktree}, commonDir: commonDir, cfg: cfg, bus: bus}, nil
}

func connect(ctx context.Context, flags *pflag.FlagSet) (cli.Handlers, func() error, error) {
	env, err := newEnvironment(ctx, flags)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	db, err := dbal.Open(ctx, env.cfg.State.DSN, env.commonDir)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	if err := db.Migrate(ctx, persistence.Module, persistence.Migrations); err != nil {
		_ = db.Close()
		return cli.Handlers{}, nil, err
	}
	gate := planGate{submit: planmodule.NewModule(env.bus, env.key.Worktree).Submit, worktree: env.key.Worktree}
	return runmodule.NewModule(db, env.bus, gate, env.cfg.Run, env.key, time.Now), db.Close, nil
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
