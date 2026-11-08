package lint

import (
	"context"

	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/lint/application/command/baseline"
	"github.com/jorge07/harness/internal/lint/application/command/initlint"
	lintcommand "github.com/jorge07/harness/internal/lint/application/command/lint"
	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/biome"
	lintconfig "github.com/jorge07/harness/internal/lint/infrastructure/config"
	"github.com/jorge07/harness/internal/lint/infrastructure/harnessengine"
	"github.com/jorge07/harness/internal/lint/infrastructure/oxlint"
	"github.com/jorge07/harness/internal/lint/infrastructure/packs"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
)

const (
	instrumentationName = "github.com/jorge07/harness/internal/lint"

	BaselinePath = ".harness-baseline/lint.json"
)

type Handlers struct {
	Lint     lintcommand.Handler
	Init     initlint.Handler
	Baseline baseline.Handler
}

func New(cfg config.Lint, version string) (Handlers, error) {
	active, settings, err := lintconfig.Decode(cfg)
	if err != nil {
		return Handlers{}, err
	}
	engines := []domain.Engine{harnessengine.New(settings), biome.New(packs.Files), oxlint.New()}
	linter := lintcommand.NewHandler(active, packs.Get, engines, version, otel.Tracer(instrumentationName), otel.Meter(instrumentationName))
	return Handlers{
		Lint: linter,
		Init: initlint.NewHandler(active, packs.Get, engines, version),
		Baseline: baseline.NewHandler(func(ctx context.Context, dir string) (shareddomain.Report, error) {
			return linter.Handle(ctx, lintcommand.Command{Dir: dir})
		}),
	}, nil
}
