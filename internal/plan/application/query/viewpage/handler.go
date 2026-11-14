// Package viewpage builds the input of the plan page and renders the page or
// one region of it. The file command and the served panel share it.
package viewpage

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	spanName      = "plan.view"
	attrProblems  = "harness.plan.problems"
	attrViewBytes = "harness.view.bytes"

	RuleBranchRead domain.RuleID = "branch-read"
	RuleProofRead  domain.RuleID = "proof-read"
)

// Query asks for the page, or for one region when Region is set. The plan
// is Path when set, else the run plan inside Worktree. The branch is read in
// Worktree, never in the worktree of the process.
type Query struct {
	Path      string
	RunID     string
	Phase     string
	LiveRound int
	Proofs    bool
	Worktree  string
	Served    *domain.ViewServed
	Region    string
}

type Check func(ctx context.Context, path string, liveRound int) (domain.Report, error)

type Dependencies struct {
	Check         Check
	Branches      domain.BranchReader
	Proofs        domain.ProofReader
	Charts        domain.ChartConverter
	Sketches      domain.SketchRenderer
	Renderer      domain.ViewRenderer
	Regions       domain.ViewRegionRenderer
	DefaultBranch func(ctx context.Context) (string, error)
	Tracer        trace.Tracer
}

type Handler struct {
	deps Dependencies
}

func NewHandler(deps Dependencies) Handler {
	return Handler{deps: deps}
}

// Handle returns the rendered bytes and the count of problems of the plan.
func (h Handler) Handle(ctx context.Context, q Query) (out []byte, problems int, err error) {
	ctx, span := h.deps.Tracer.Start(ctx, spanName)
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.SetAttributes(attribute.Int(attrProblems, problems), attribute.Int(attrViewBytes, len(out)))
		span.End()
	}()

	path := q.Path
	if path == "" {
		path = filepath.Join(q.Worktree, filepath.FromSlash(shareddomain.RunPlanFile(q.RunID)))
	}
	report, err := h.deps.Check(ctx, path, q.LiveRound)
	if err != nil {
		return nil, 0, err
	}
	if report.Map != nil {
		return nil, len(report.Problems), domain.ErrMapNoPage
	}
	var in domain.ViewInput
	if report.Program != nil {
		in = domain.ViewInput{Program: report.Program, Problems: report.Problems}
	} else {
		in = domain.ViewInput{
			Doc:      report.Doc,
			Problems: report.Problems,
			Links:    domain.Trace(report.Doc),
			Run:      domain.ViewRun{ID: q.RunID, Phase: q.Phase, Track: string(report.Track), Branch: report.Doc.RunPR().Branch},
			Charts:   h.charts(report.Doc),
			Sketches: h.sketches(report.Doc, filepath.Dir(path)),
		}
		h.addEvidence(ctx, q, &in)
	}
	in.Served = q.Served
	problems = len(in.Problems)

	if q.Region != "" {
		if h.deps.Regions == nil {
			return nil, problems, domain.ErrNoRegions
		}
		out, err = h.deps.Regions.RenderRegion(in, q.Region)
	} else {
		out, err = h.deps.Renderer.Render(in)
	}
	if err != nil {
		return nil, problems, err
	}
	return out, problems, nil
}

func (h Handler) addEvidence(ctx context.Context, q Query, in *domain.ViewInput) {
	base := in.Doc.Base()
	if base == "" {
		var err error
		if base, err = h.deps.DefaultBranch(ctx); err != nil {
			in.Problems = append(in.Problems, readProblem(RuleBranchRead, err))
		}
	}
	if base != "" {
		branch, err := h.deps.Branches.Read(ctx, q.Worktree, base, domain.TestFiles(in.Doc))
		if err != nil {
			in.Problems = append(in.Problems, readProblem(RuleBranchRead, err))
		} else {
			in.Branch = &branch
		}
	}
	if q.Proofs {
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
