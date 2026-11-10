package plan

import (
	"context"

	"go.opentelemetry.io/otel"

	"github.com/90p-labs/harness/internal/plan/application/command/renderview"
	"github.com/90p-labs/harness/internal/plan/application/command/submitplan"
	"github.com/90p-labs/harness/internal/plan/application/query/checkplan"
	"github.com/90p-labs/harness/internal/plan/application/query/markdiagram"
	"github.com/90p-labs/harness/internal/plan/domain"
	"github.com/90p-labs/harness/internal/plan/infrastructure/browser"
	"github.com/90p-labs/harness/internal/plan/infrastructure/excalidraw"
	"github.com/90p-labs/harness/internal/plan/infrastructure/gitdiff"
	"github.com/90p-labs/harness/internal/plan/infrastructure/mermaid"
	"github.com/90p-labs/harness/internal/plan/infrastructure/reviewplugin"
	"github.com/90p-labs/harness/internal/plan/infrastructure/view"
	"github.com/90p-labs/harness/internal/plan/ui/cli"
	"github.com/90p-labs/harness/internal/plan/ui/viewplugin"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const tracerName = "github.com/90p-labs/harness/internal/plan"

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

var opener = browser.New()

func NewViewPlugin(worktree string, proofs domain.ProofReader, defaultBranch DefaultBranch, liveRound viewplugin.LiveRound) shareddomain.Plugin {
	return viewplugin.New(worktree, newView(newCheck(), worktree, proofs, defaultBranch).Handle, liveRound, opener.Open)
}

func NewModule(bus shareddomain.Dispatcher, worktree string, proofs domain.ProofReader, defaultBranch DefaultBranch) cli.Handlers {
	check := newCheck()
	return cli.Handlers{
		Check:  check,
		Submit: submitplan.NewHandler(checkFile(check), bus, worktree),
		View:   newView(check, worktree, proofs, defaultBranch),
		Open:   opener.Open,
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
