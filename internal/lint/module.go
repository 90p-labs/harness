package lint

import (
	"context"
	"slices"

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
	Lint        lintcommand.Handler
	Canaries    canaries.Handler
	Init        initlint.Handler
	Baseline    baseline.Handler
	PluginRules []domain.Rule
}

func splitDisabled(disabled []string, defaults []domain.Engine) (engines, rules []string) {
	for _, entry := range disabled {
		if slices.ContainsFunc(defaults, func(engine domain.Engine) bool { return string(engine.Name()) == entry }) {
			engines = append(engines, entry)
		} else {
			rules = append(rules, entry)
		}
	}
	return engines, rules
}

func New(cfg config.Lint, plugins []config.Plugin, worktree, version string, render harnessengine.RenderChart) (Handlers, error) {
	ruleCatalogue, err := rulecatalogue.Load()
	if err != nil {
		return Handlers{}, err
	}
	catalogue, active, err := packs.LoadSharing(worktree, cfg.Packs, ruleCatalogue)
	if err != nil {
		return Handlers{}, err
	}
	version = catalogue.Version(version)
	execs, err := execplugin.FromConfig(plugins)
	if err != nil {
		return Handlers{}, err
	}
	defaults := []domain.Engine{harnessengine.New(lintconfig.Decode(cfg), render), biome.New(catalogue.Files()), oxlint.New()}
	disabledEngines, disabledRules := splitDisabled(cfg.Disabled, defaults)
	engines, err := lintplugin.NewEngines(defaults, disabledEngines, execs, git.Files, otel.GetTracerProvider(), otel.GetMeterProvider())
	if err != nil {
		return Handlers{}, err
	}
	lookup, err := domain.WithoutRules(active, catalogue.Get, disabledRules)
	if err != nil {
		return Handlers{}, err
	}
	rules, err := domain.RulesByEngine(active, lookup)
	if err != nil {
		return Handlers{}, err
	}
	if err := engines.Cover(rules); err != nil {
		return Handlers{}, err
	}
	linter := lintcommand.NewHandler(active, lookup, engines, version, otel.Tracer(instrumentationName), otel.Meter(instrumentationName))
	return Handlers{
		Lint:     linter,
		Canaries: canaries.NewHandler(ruleCatalogue, manifest.Read, rules, engines, linter.CheckVersion),
		Init:     initlint.NewHandler(active, lookup, engines, version),
		Baseline: baseline.NewHandler(func(ctx context.Context, dir string) (shareddomain.Report, error) {
			return linter.Handle(ctx, lintcommand.Command{Dir: dir})
		}),
		PluginRules: ruleCatalogue.PluginRules(rules),
	}, nil
}
