package live

import (
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
)

const instrumentationName = "github.com/90p-labs/harness/internal/live"

func NewCheck(db *dbal.DB, runs domain.Runs, bus shareddomain.Dispatcher, cfg config.Live) check.Handler {
	return check.NewHandler(runs, persistence.NewSQLReleaseRepository(db), github.New(gh.Binary), archive.New(), bus,
		cfg.Environment, cfg.Rollback, otel.Tracer(instrumentationName), otel.Meter(instrumentationName))
}

func NewSessionPlugin(lines sessionplugin.Lines) shareddomain.Plugin {
	return sessionplugin.New(lines)
}
