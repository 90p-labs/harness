package renderview

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/plan/domain"
)

const (
	spanName      = "plan.view"
	attrProblems  = "harness.plan.problems"
	attrViewBytes = "harness.view.bytes"
	pageFile      = "plan.html"
	pageMode      = 0o600

	RuleBranchRead domain.RuleID = "branch-read"
	RuleProofRead  domain.RuleID = "proof-read"
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
	deps Dependencies
}

func NewHandler(deps Dependencies) Handler {
	return Handler{deps: deps}
}

func (h Handler) Handle(ctx context.Context, c Command) (out string, err error) {
	ctx, span := h.deps.Tracer.Start(ctx, spanName)
	problems, size := 0, 0
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.SetAttributes(attribute.Int(attrProblems, problems), attribute.Int(attrViewBytes, size))
		span.End()
	}()

	report, err := h.deps.Check(ctx, c.Path, c.LiveRound)
	if err != nil {
		return "", err
	}
	in := domain.ViewInput{
		Doc:      report.Doc,
		Problems: report.Problems,
		Links:    domain.Trace(report.Doc),
		Run:      domain.ViewRun{ID: c.RunID, Phase: c.Phase, Track: string(report.Track), Branch: report.Doc.RunPR().Branch},
		Charts:   h.charts(report.Doc),
		Sketches: h.sketches(report.Doc, filepath.Dir(c.Path)),
	}
	h.addEvidence(ctx, c, &in)
	problems = len(in.Problems)

	page, err := h.deps.Renderer.Render(in)
	if err != nil {
		return "", err
	}
	size = len(page)
	out = c.Out
	if out == "" {
		out = filepath.Join(filepath.Dir(c.Path), pageFile)
	}
	if err := os.WriteFile(out, page, pageMode); err != nil {
		return "", err
	}
	return out, nil
}

func (h Handler) addEvidence(ctx context.Context, c Command, in *domain.ViewInput) {
	base := in.Doc.Base()
	if base == "" {
		var err error
		if base, err = h.deps.DefaultBranch(ctx); err != nil {
			in.Problems = append(in.Problems, readProblem(RuleBranchRead, err))
		}
	}
	if base != "" {
		branch, err := h.deps.Branches.Read(ctx, h.deps.Worktree, base, domain.TestFiles(in.Doc))
		if err != nil {
			in.Problems = append(in.Problems, readProblem(RuleBranchRead, err))
		} else {
			in.Branch = &branch
		}
	}
	if c.Proofs {
		proofs, err := h.deps.Proofs.Passed(ctx)
		if err != nil {
			in.Problems = append(in.Problems, readProblem(RuleProofRead, err))
		}
		in.Proofs = proofs
	}
}

func readProblem(rule domain.RuleID, err error) domain.Problem {
	return domain.Problem{Rule: rule, Message: err.Error()}
}

func (h Handler) charts(doc domain.Document) map[int]string {
	charts := map[int]string{}
	for _, d := range doc.Diagrams {
		if d.Kind != domain.DiagramState {
			continue
		}
		if chart, err := h.deps.Charts.StateChart(d.Body); err == nil {
			charts[d.Line] = chart
		}
	}
	return charts
}

func (h Handler) sketches(doc domain.Document, dir string) map[string][]byte {
	sketches := map[string][]byte{}
	for _, s := range doc.Sketches {
		scene, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(s.Path)))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		var svg []byte
		if err == nil {
			svg, err = h.deps.Sketches.RenderSVG(scene)
		}
		if err != nil {
			svg = []byte(err.Error())
		}
		sketches[s.Path] = svg
	}
	return sketches
}
