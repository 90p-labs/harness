package live

import (
	"context"

	"go.opentelemetry.io/otel"

	"github.com/90p-labs/harness/internal/live/application/command/check"
	"github.com/90p-labs/harness/internal/live/domain"
	"github.com/90p-labs/harness/internal/live/infrastructure/archive"
	"github.com/90p-labs/harness/internal/live/infrastructure/github"
	"github.com/90p-labs/harness/internal/live/infrastructure/persistence"
	"github.com/90p-labs/harness/internal/live/infrastructure/sessionplugin"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
	gh "github.com/90p-labs/harness/internal/shared/infrastructure/github"
	"github.com/90p-labs/harness/internal/shared/infrastructure/telemetry"
)

const instrumentationName = "github.com/90p-labs/harness/internal/live"

func NewCheck(db *dbal.DB, runs domain.Runs, bus shareddomain.Dispatcher, cfg config.Live) check.Handler {
	return check.NewHandler(runs, persistence.NewSQLReleaseRepository(db), github.New(gh.Binary), archive.New(), bus,
		cfg.Environment, cfg.Rollback, otel.Tracer(instrumentationName), runTrace{}, otel.Meter(instrumentationName))
}

type runTrace struct{}

func (runTrace) Under(ctx context.Context, run domain.LiveRun) context.Context {
	return telemetry.UnderRun(ctx, run.ID)
}

func NewSessionPlugin(lines sessionplugin.Lines) shareddomain.Plugin {
	return sessionplugin.New(lines)
}
