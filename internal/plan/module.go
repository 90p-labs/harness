package plan

import (
	"context"

	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/plan/application/command/renderview"
	"github.com/jorge07/harness/internal/plan/application/command/submitplan"
	"github.com/jorge07/harness/internal/plan/application/query/checkplan"
	"github.com/jorge07/harness/internal/plan/application/query/markdiagram"
	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/plan/infrastructure/excalidraw"
	"github.com/jorge07/harness/internal/plan/infrastructure/gitdiff"
	"github.com/jorge07/harness/internal/plan/infrastructure/mermaid"
	"github.com/jorge07/harness/internal/plan/infrastructure/reviewplugin"
	"github.com/jorge07/harness/internal/plan/infrastructure/view"
	"github.com/jorge07/harness/internal/plan/ui/cli"
	"github.com/jorge07/harness/internal/plan/ui/viewplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const tracerName = "github.com/jorge07/harness/internal/plan"

type DefaultBranch func(ctx context.Context) (string, error)

type chartConverter struct{}

func (chartConverter) StateChart(body string) (string, error) { return mermaid.StateChart(body) }

type sketchRenderer struct{}

func (sketchRenderer) RenderSVG(scene []byte) ([]byte, error) { return excalidraw.RenderSVG(scene) }

func NewDiagram() markdiagram.Handler {
	return markdiagram.NewHandler(mermaid.Parser{}, otel.Tracer(tracerName))
}

func NewReviewPlugin(worktree string, proofs domain.ProofReader, defaultBranch DefaultBranch) shareddomain.Plugin {
	return reviewplugin.New(worktree, gitdiff.Reader{}, proofs, defaultBranch)
}

func NewViewPlugin(worktree string, proofs domain.ProofReader, defaultBranch DefaultBranch, liveRound viewplugin.LiveRound) shareddomain.Plugin {
	return viewplugin.New(worktree, newView(newCheck(), worktree, proofs, defaultBranch).Handle, liveRound)
}

func NewModule(bus shareddomain.Dispatcher, worktree string, proofs domain.ProofReader, defaultBranch DefaultBranch) cli.Handlers {
	check := newCheck()
	return cli.Handlers{
		Check:  check,
		Submit: submitplan.NewHandler(checkFile(check), bus, worktree),
		View:   newView(check, worktree, proofs, defaultBranch),
	}
}

func newCheck() checkplan.Handler {
	return checkplan.NewHandler(mermaid.Parser{}, otel.Tracer(tracerName))
}

func checkFile(check checkplan.Handler) func(ctx context.Context, path string, liveRound int) (domain.Report, error) {
	return func(ctx context.Context, path string, liveRound int) (domain.Report, error) {
		return check.Handle(ctx, checkplan.Query{Path: path, LiveRound: liveRound})
	}
}

func newView(check checkplan.Handler, worktree string, proofs domain.ProofReader, defaultBranch DefaultBranch) renderview.Handler {
	return renderview.NewHandler(renderview.Dependencies{
		Check:         checkFile(check),
		Branches:      gitdiff.Reader{},
		Proofs:        proofs,
		Charts:        chartConverter{},
		Sketches:      sketchRenderer{},
		Renderer:      view.NewRenderer(),
		DefaultBranch: defaultBranch,
		Worktree:      worktree,
		Tracer:        otel.Tracer(tracerName),
	})
}
