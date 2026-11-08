package main

import (
	"context"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	runmodule "github.com/jorge07/harness/internal/run"
	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/run/infrastructure/persistence"
	"github.com/jorge07/harness/internal/run/ui/cli"
	"github.com/jorge07/harness/internal/shared/application/eventbus"
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

func connect(ctx context.Context, flags *pflag.FlagSet) (cli.Handlers, func() error, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	repo, worktree, err := git.RepoKey(ctx, cwd)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	commonDir, err := git.CommonDir(ctx, cwd)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	cfg, err := config.Load(flags, worktree)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	db, err := dbal.Open(ctx, cfg.State.DSN, commonDir)
	if err != nil {
		return cli.Handlers{}, nil, err
	}
	if err := db.Migrate(ctx, persistence.Module, persistence.Migrations); err != nil {
		_ = db.Close()
		return cli.Handlers{}, nil, err
	}
	plugins, err := execplugin.FromConfig(cfg.Plugins)
	if err != nil {
		_ = db.Close()
		return cli.Handlers{}, nil, err
	}
	bus := eventbus.NewBus(plugins, otel.GetTracerProvider(), otel.GetMeterProvider())
	handlers := runmodule.NewModule(db, bus, cfg.Run, domain.RunKey{Repo: repo, Worktree: worktree}, time.Now)
	return handlers, db.Close, nil
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
