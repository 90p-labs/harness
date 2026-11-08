package lint

import (
	"context"

	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/lint/application/command/baseline"
	"github.com/jorge07/harness/internal/lint/application/command/canaries"
	"github.com/jorge07/harness/internal/lint/application/command/initlint"
	lintcommand "github.com/jorge07/harness/internal/lint/application/command/lint"
	"github.com/jorge07/harness/internal/lint/application/lintplugin"
	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/biome"
	lintconfig "github.com/jorge07/harness/internal/lint/infrastructure/config"
	"github.com/jorge07/harness/internal/lint/infrastructure/harnessengine"
	"github.com/jorge07/harness/internal/lint/infrastructure/manifest"
	"github.com/jorge07/harness/internal/lint/infrastructure/oxlint"
	"github.com/jorge07/harness/internal/lint/infrastructure/packs"
	"github.com/jorge07/harness/internal/lint/infrastructure/rulecatalogue"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/execplugin"
	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

const (
	instrumentationName = "github.com/jorge07/harness/internal/lint"

	BaselinePath = ".harness-baseline/lint.json"
)

type Handlers struct {
	Lint     lintcommand.Handler
	Canaries canaries.Handler
	Init     initlint.Handler
	Baseline baseline.Handler
}

func New(cfg config.Lint, plugins []config.Plugin, worktree, version string, render harnessengine.RenderChart) (Handlers, error) {
	catalogue, active, err := packs.Load(worktree, cfg.Packs)
	if err != nil {
		return Handlers{}, err
	}
	version = catalogue.Version(version)
	execs, err := execplugin.FromConfig(plugins)
	if err != nil {
		return Handlers{}, err
	}
	defaults := []domain.Engine{harnessengine.New(lintconfig.Decode(cfg), render), biome.New(catalogue.Files()), oxlint.New()}
	engines, err := lintplugin.NewEngines(defaults, cfg.Disabled, execs, git.Files, otel.GetTracerProvider(), otel.GetMeterProvider())
	if err != nil {
		return Handlers{}, err
	}
	rules, err := domain.RulesByEngine(active, catalogue.Get)
	if err != nil {
		return Handlers{}, err
	}
	if err := engines.Cover(rules); err != nil {
		return Handlers{}, err
	}
	ruleCatalogue, err := rulecatalogue.Load()
	if err != nil {
		return Handlers{}, err
	}
	linter := lintcommand.NewHandler(active, catalogue.Get, engines, version, otel.Tracer(instrumentationName), otel.Meter(instrumentationName))
	return Handlers{
		Lint:     linter,
		Canaries: canaries.NewHandler(ruleCatalogue, manifest.Read, rules, engines, linter.CheckVersion),
		Init:     initlint.NewHandler(active, catalogue.Get, engines, version),
		Baseline: baseline.NewHandler(func(ctx context.Context, dir string) (shareddomain.Report, error) {
			return linter.Handle(ctx, lintcommand.Command{Dir: dir})
		}),
	}, nil
}
