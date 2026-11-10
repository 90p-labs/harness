package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"slices"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.opentelemetry.io/otel"

	"github.com/90p-labs/harness/internal/deploy"
	"github.com/90p-labs/harness/internal/guard"
	livemodule "github.com/90p-labs/harness/internal/live"
	livecli "github.com/90p-labs/harness/internal/live/ui/cli"
	planmodule "github.com/90p-labs/harness/internal/plan"
	"github.com/90p-labs/harness/internal/plan/application/command/submitplan"
	runmodule "github.com/90p-labs/harness/internal/run"
	"github.com/90p-labs/harness/internal/run/application/command/recordapproval"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/ui/approvalplugin"
	"github.com/90p-labs/harness/internal/run/ui/cli"
	"github.com/90p-labs/harness/internal/shared/application/eventbus"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
	"github.com/90p-labs/harness/internal/shared/infrastructure/execplugin"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
	"github.com/90p-labs/harness/internal/shared/infrastructure/telemetry"
)

func newRunCommand(flags *pflag.FlagSet) *cobra.Command {
	cmd := cli.NewCommand(func(ctx context.Context) (cli.Handlers, func() error, error) {
		return connect(ctx, flags)
	})
	cmd.RunE = func(*cobra.Command, []string) error { return usageError{errNoCommand} }
	return cmd
}

type runRefKey struct{}

func runRefFrom(ctx context.Context) shareddomain.RunRef {
	ref, _ := ctx.Value(runRefKey{}).(shareddomain.RunRef)
	return ref
}

// resolveRun never fails: a hook must not fail on telemetry, so a step that fails keeps what is known.
func resolveRun(cmd *cobra.Command) {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	repo, worktree, err := git.RepoKey(ctx, cwd)
	if err != nil {
		return
	}
	ref := resolveActiveRun(ctx, cmd.Root().PersistentFlags(), cwd, domain.RunKey{Repo: repo, Worktree: worktree})
	cmd.SetContext(context.WithValue(telemetry.UnderRun(telemetry.WithRun(ctx, worktree, ref.ID), ref.ID), runRefKey{}, ref))
}

func resolveActiveRun(ctx context.Context, flags *pflag.FlagSet, cwd string, key domain.RunKey) shareddomain.RunRef {
	commonDir, err := git.CommonDir(ctx, cwd)
	if err != nil {
		return shareddomain.RunRef{}
	}
	cfg, err := config.Load(flags, key.Worktree)
	if err != nil || !dbal.FileExists(cfg.State.DSN, commonDir) {
		return shareddomain.RunRef{}
	}
	db, err := openState(ctx, cfg, commonDir)
	if err != nil {
		return shareddomain.RunRef{}
	}
	defer func() { _ = db.Close() }()
	current, err := runmodule.NewStatus(db, time.Now).Handle(ctx, status.Query{Key: key})
	if err != nil {
		return shareddomain.RunRef{}
	}
	return shareddomain.RunRef{ID: current.ID, Phase: string(current.Phase)}
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
	deployPlugins, err := deploy.NewPlugins(cfg.Deploy.Disabled, plugins)
	if err != nil {
		return environment{}, err
	}
	key := domain.RunKey{Repo: repo, Worktree: worktree}
	guards, err := guard.NewPlugins(cfg.Guard.Disabled, cfg.Git.DefaultBranch, modelsByHost(cfg.Roles), worktree,
		stopAdapter{cfg: cfg, commonDir: commonDir, key: key},
		commitCheckAdapter{cfg: cfg}.Passed,
		reviewRecordAdapter{cfg: cfg}.Save,
		reviewRecordAdapter{cfg: cfg}.Models)
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
	approval := approvalplugin.New(approvalAdapter{cfg: cfg, commonDir: commonDir, worktree: worktree}.Record)
	bus = eventbus.NewBus(slices.Concat(plugins, guards, deployPlugins, []shareddomain.Plugin{review, planView, session, approval}), otel.GetTracerProvider(), otel.GetMeterProvider()).WithTraceDetail(telemetry.TraceDetailFromEnv())
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

func (g planGate) Submit(ctx context.Context, run domain.Run) (domain.Submission, error) {
	result, err := g.submit.Handle(ctx, submitplan.Command{
		RunID: run.ID, Phase: string(run.Phase),
		Path: planPath(g.worktree, run.ID), LiveRound: run.LiveRound,
	})
	return domain.Submission{Track: result.Track, ApprovalWaived: result.ApprovalWaived, QuestionsOpen: result.QuestionsOpen}, err
}

func (g planGate) Hash(run domain.Run) (string, error) {
	content, err := os.ReadFile(planPath(g.worktree, run.ID))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]), nil
}

type approvalAdapter struct {
	cfg       config.Config
	commonDir string
	worktree  string
}

func (a approvalAdapter) Record(ctx context.Context, runID string) error {
	db, err := openState(ctx, a.cfg, a.commonDir)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	return runmodule.NewApprovalRecorder(db, planGate{worktree: a.worktree}, time.Now).Handle(ctx, recordapproval.Command{RunID: runID})
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
