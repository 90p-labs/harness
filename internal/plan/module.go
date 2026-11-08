package plan

import (
	"context"

	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/plan/application/command/submitplan"
	"github.com/jorge07/harness/internal/plan/application/query/checkplan"
	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/plan/infrastructure/mermaid"
	"github.com/jorge07/harness/internal/plan/ui/cli"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const tracerName = "github.com/jorge07/harness/internal/plan"

func NewModule(bus shareddomain.Dispatcher, worktree string) cli.Handlers {
	check := checkplan.NewHandler(mermaid.Parser{}, otel.Tracer(tracerName))
	checkFile := func(ctx context.Context, path string, liveRound int) (domain.Report, error) {
		return check.Handle(ctx, checkplan.Query{Path: path, LiveRound: liveRound})
	}
	return cli.Handlers{
		Check:  check,
		Submit: submitplan.NewHandler(checkFile, bus, worktree),
	}
}
