package renderview

import (
	"context"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/plan/application/query/viewpage"
	"github.com/90p-labs/harness/internal/plan/domain"
)

const (
	pageFile = "plan.html"
	pageMode = 0o600

	RuleBranchRead = viewpage.RuleBranchRead
	RuleProofRead  = viewpage.RuleProofRead
)

type Command struct {
	Path      string
	Out       string
	RunID     string
	Phase     string
	LiveRound int
	Proofs    bool
}

type Check func(ctx context.Context, path string, liveRound int) (domain.Report, error)

type Dependencies struct {
	Check         Check
	Branches      domain.BranchReader
	Proofs        domain.ProofReader
	Charts        domain.ChartConverter
	Sketches      domain.SketchRenderer
	Renderer      domain.ViewRenderer
	DefaultBranch func(ctx context.Context) (string, error)
	Worktree      string
	Tracer        trace.Tracer
}

type Handler struct {
	page     viewpage.Handler
	worktree string
}

func NewHandler(deps Dependencies) Handler {
	return Handler{page: viewpage.NewHandler(viewpage.Dependencies{
		Check:         viewpage.Check(deps.Check),
		Branches:      deps.Branches,
		Proofs:        deps.Proofs,
		Charts:        deps.Charts,
		Sketches:      deps.Sketches,
		Renderer:      deps.Renderer,
		DefaultBranch: deps.DefaultBranch,
		Tracer:        deps.Tracer,
	}), worktree: deps.Worktree}
}

func (h Handler) Handle(ctx context.Context, c Command) (string, error) {
	page, _, err := h.page.Handle(ctx, viewpage.Query{
		Path:      c.Path,
		RunID:     c.RunID,
		Phase:     c.Phase,
		LiveRound: c.LiveRound,
		Proofs:    c.Proofs,
		Worktree:  h.worktree,
	})
	if err != nil {
		return "", err
	}
	out := c.Out
	if out == "" {
		out = filepath.Join(filepath.Dir(c.Path), pageFile)
	}
	if err := os.WriteFile(out, page, pageMode); err != nil {
		return "", err
	}
	return out, nil
}
