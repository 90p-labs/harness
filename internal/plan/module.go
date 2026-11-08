package plan

import (
	"context"

	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/plan/application/command/submitplan"
	"github.com/jorge07/harness/internal/plan/application/query/checkplan"
	"github.com/jorge07/harness/internal/plan/application/query/markdiagram"
	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/plan/infrastructure/gitdiff"
	"github.com/jorge07/harness/internal/plan/infrastructure/mermaid"
	"github.com/jorge07/harness/internal/plan/infrastructure/reviewplugin"
	"github.com/jorge07/harness/internal/plan/ui/cli"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const tracerName = "github.com/jorge07/harness/internal/plan"

func NewDiagram() markdiagram.Handler {
	return markdiagram.NewHandler(mermaid.Parser{}, otel.Tracer(tracerName))
}

func NewReviewPlugin(worktree string, proofs domain.ProofReader, defaultBranch func(ctx context.Context) (string, error)) shareddomain.Plugin {
	return reviewplugin.New(worktree, gitdiff.Reader{}, proofs, defaultBranch)
}

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
