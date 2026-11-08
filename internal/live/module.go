package live

import (
	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/live/application/command/check"
	"github.com/jorge07/harness/internal/live/domain"
	"github.com/jorge07/harness/internal/live/infrastructure/archive"
	"github.com/jorge07/harness/internal/live/infrastructure/github"
	"github.com/jorge07/harness/internal/live/infrastructure/persistence"
	"github.com/jorge07/harness/internal/live/infrastructure/sessionplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/dbal"
	gh "github.com/jorge07/harness/internal/shared/infrastructure/github"
)

const instrumentationName = "github.com/jorge07/harness/internal/live"

func NewCheck(db *dbal.DB, runs domain.Runs, bus shareddomain.Dispatcher, cfg config.Live) check.Handler {
	return check.NewHandler(runs, persistence.NewSQLReleaseRepository(db), github.New(gh.Binary), archive.New(), bus,
		cfg.Environment, cfg.Rollback, otel.Tracer(instrumentationName), otel.Meter(instrumentationName))
}

func NewSessionPlugin(lines sessionplugin.Lines) shareddomain.Plugin {
	return sessionplugin.New(lines)
}
