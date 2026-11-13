package viewpage_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/plan/application/query/checkplan"
	"github.com/90p-labs/harness/internal/plan/application/query/viewpage"
	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	validPlan  = "../../../domain/testdata/valid.md"
	runID      = "run-9"
	pageBody   = "<html>page</html>"
	regionBody = "<section>region</section>"
)

type fakeRenderer struct {
	page, region domain.ViewInput
	name         string
}

func (f *fakeRenderer) Render(in domain.ViewInput) ([]byte, error) {
	f.page = in
	return []byte(pageBody), nil
}

func (f *fakeRenderer) RenderRegion(in domain.ViewInput, name string) ([]byte, error) {
	f.region, f.name = in, name
	return []byte(regionBody), nil
}

type fakeBranches struct{ worktree string }

func (f *fakeBranches) Read(_ context.Context, worktree, _ string, _ []string) (domain.Branch, error) {
	f.worktree = worktree
	return domain.Branch{}, nil
}

type fakeProofs struct{ called bool }

func (f *fakeProofs) Passed(context.Context) (map[int]bool, error) {
	f.called = true
	return map[int]bool{1: true}, nil
}

type noCharts struct{}

func (noCharts) StateChart(string) (string, error) { return "", errors.New("none") }

type noSketches struct{}

func (noSketches) RenderSVG([]byte) ([]byte, error) { return nil, errors.New("none") }

type fixture struct {
	handler  viewpage.Handler
	renderer *fakeRenderer
	branches *fakeBranches
	proofs   *fakeProofs
	spans    *tracetest.SpanRecorder
	root     string
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	f := fixture{
		renderer: &fakeRenderer{},
		branches: &fakeBranches{},
		proofs:   &fakeProofs{},
		spans:    tracetest.NewSpanRecorder(),
		root:     t.TempDir(),
	}
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(f.spans)).Tracer("test")
	check := checkplan.NewHandler(nil, tracer)
	f.handler = viewpage.NewHandler(viewpage.Dependencies{
		Check: func(ctx context.Context, path string, liveRound int) (domain.Report, error) {
			return check.Handle(ctx, checkplan.Query{Path: path, LiveRound: liveRound})
		},
		Branches:      f.branches,
		Proofs:        f.proofs,
		Charts:        noCharts{},
		Sketches:      noSketches{},
		Renderer:      f.renderer,
		Regions:       f.renderer,
		DefaultBranch: func(context.Context) (string, error) { return "trunk", nil },
		Tracer:        tracer,
	})
	return f
}

func (f fixture) worktree(t *testing.T, name string) string {
	t.Helper()
	valid, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(f.root, name)
	file := filepath.Join(dir, filepath.FromSlash(shareddomain.RunPlanFile(runID)))
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, valid, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestPageUsesRunWorktree(t *testing.T) {
	f := newFixture(t)
	panelTree := f.root + "/panel-worktree-without-a-plan"
	runTree := f.worktree(t, "run-worktree")
	served := &domain.ViewServed{Version: "p.r", Base: "/runs/run-9/regions/"}

	page, problems, err := f.handler.Handle(t.Context(), viewpage.Query{
		RunID: runID, Phase: "planning", Worktree: runTree, Served: served, Proofs: true,
	})

	if err != nil {
		t.Fatalf("Handle: %v (panel worktree %s)", err, panelTree)
	}
	if string(page) != pageBody {
		t.Errorf("page = %q, want %q", page, pageBody)
	}
	if f.branches.worktree != runTree {
		t.Errorf("branch read from %q, want %q", f.branches.worktree, runTree)
	}
	if f.renderer.page.Served != served || f.renderer.page.Run.ID != runID {
		t.Errorf("input = %+v", f.renderer.page)
	}
	if !f.proofs.called || !f.renderer.page.Proofs[1] {
		t.Error("proofs were not read")
	}
	if problems != len(f.renderer.page.Problems) {
		t.Errorf("problems = %d, want %d", problems, len(f.renderer.page.Problems))
	}
}

func TestPageRegion(t *testing.T) {
	f := newFixture(t)
	tree := f.worktree(t, "wt")

	got, _, err := f.handler.Handle(t.Context(), viewpage.Query{RunID: runID, Worktree: tree, Region: "criteria"})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if string(got) != regionBody || f.renderer.name != "criteria" {
		t.Errorf("got %q for region %q", got, f.renderer.name)
	}
}

func TestPageSpan(t *testing.T) {
	f := newFixture(t)
	tree := f.worktree(t, "wt")

	if _, _, err := f.handler.Handle(t.Context(), viewpage.Query{RunID: runID, Worktree: tree}); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	var found int
	for _, s := range f.spans.Ended() {
		if s.Name() != "plan.view" {
			continue
		}
		found++
		keys := map[attribute.Key]bool{}
		for _, kv := range s.Attributes() {
			keys[kv.Key] = true
		}
		if !keys["harness.plan.problems"] || !keys["harness.view.bytes"] {
			t.Errorf("attributes = %v", s.Attributes())
		}
	}
	if found != 1 {
		t.Errorf("plan.view spans = %d, want 1", found)
	}
}
