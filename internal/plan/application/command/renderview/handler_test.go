package renderview_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/plan/application/command/renderview"
	"github.com/90p-labs/harness/internal/plan/application/query/checkplan"
	"github.com/90p-labs/harness/internal/plan/domain"
)

const (
	validPlan   = "../../../domain/testdata/valid.md"
	notesPlan   = "\n## Notes\n\n```mermaid\nstateDiagram-v2\n    a --> b\n```\n\n```mermaid\nstateDiagram-v2\n    note right of a : x\n```\n\n![Good](ui/good.excalidraw)\n![Bad](ui/bad.excalidraw)\n![Lost](ui/lost.excalidraw)\n![Locked](ui/locked.excalidraw)\n"
	goodScene   = `{"good":true}`
	badScene    = `{"bad":true}`
	pageBody    = "<html>page</html>"
	branchError = "git exploded"
	proofError  = "no database"
	chartMark   = "converted"
	sceneSVG    = "<svg>ok</svg>"
	sceneError  = "unknown element e1"
)

type fakeRenderer struct{ input domain.ViewInput }

func (f *fakeRenderer) Render(in domain.ViewInput) ([]byte, error) {
	f.input = in
	return []byte(pageBody), nil
}

type fakeBranches struct {
	branch domain.Branch
	err    error
	base   string
}

func (f *fakeBranches) Read(_ context.Context, _, base string, _ []string) (domain.Branch, error) {
	f.base = base
	return f.branch, f.err
}

type fakeProofs struct {
	passed map[int]bool
	err    error
	called bool
}

func (f *fakeProofs) Passed(context.Context) (map[int]bool, error) {
	f.called = true
	return f.passed, f.err
}

type fakeCharts struct{}

func (fakeCharts) StateChart(body string) (string, error) {
	if strings.Contains(body, "note") {
		return "", errors.New("state diagram has no flowchart form")
	}
	return chartMark, nil
}

type fakeSketches struct{}

func (fakeSketches) RenderSVG(scene []byte) ([]byte, error) {
	if string(scene) == badScene {
		return nil, errors.New(sceneError)
	}
	return []byte(sceneSVG), nil
}

type fixture struct {
	handler  renderview.Handler
	renderer *fakeRenderer
	branches *fakeBranches
	proofs   *fakeProofs
	spans    *tracetest.SpanRecorder
	dir      string
	plan     string
}

func newFixture(t *testing.T, defaultBranch func(context.Context) (string, error)) fixture {
	t.Helper()
	f := fixture{
		renderer: &fakeRenderer{},
		branches: &fakeBranches{},
		proofs:   &fakeProofs{passed: map[int]bool{1: true}},
		spans:    tracetest.NewSpanRecorder(),
		dir:      t.TempDir(),
	}
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(f.spans)).Tracer("test")
	check := checkplan.NewHandler(nil, tracer)
	f.handler = renderview.NewHandler(renderview.Dependencies{
		Check: func(ctx context.Context, path string, liveRound int) (domain.Report, error) {
			return check.Handle(ctx, checkplan.Query{Path: path, LiveRound: liveRound})
		},
		Branches:      f.branches,
		Proofs:        f.proofs,
		Charts:        fakeCharts{},
		Sketches:      fakeSketches{},
		Renderer:      f.renderer,
		DefaultBranch: defaultBranch,
		Tracer:        tracer,
	})
	valid, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	f.plan = filepath.Join(f.dir, "plan.md")
	write(t, f.plan, string(valid)+notesPlan)
	write(t, filepath.Join(f.dir, "ui", "good.excalidraw"), goodScene)
	write(t, filepath.Join(f.dir, "ui", "bad.excalidraw"), badScene)
	return f
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func trunkBranch(context.Context) (string, error) { return "trunk", nil }

func viewSpan(t *testing.T, f fixture) []attribute.KeyValue {
	t.Helper()
	var found []sdktrace.ReadOnlySpan
	for _, s := range f.spans.Ended() {
		if s.Name() == "plan.view" {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		t.Fatalf("plan.view spans = %d, want 1", len(found))
	}
	return found[0].Attributes()
}

func TestRenderViewSpan(t *testing.T) {
	t.Run("a clean read writes the page beside the plan and records the span", func(t *testing.T) {
		f := newFixture(t, trunkBranch)
		f.branches.branch = domain.Branch{Files: []domain.FileChange{{Path: "a.go", Status: domain.StatusAdded}}}

		out, err := f.handler.Handle(t.Context(), renderview.Command{Path: f.plan, RunID: "run-1", Phase: "planning", Proofs: true})

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if want := filepath.Join(f.dir, "plan.html"); out != want {
			t.Errorf("out = %q, want %q", out, want)
		}
		if got, _ := os.ReadFile(out); string(got) != pageBody {
			t.Errorf("page = %q, want %q", got, pageBody)
		}
		in := f.renderer.input
		if f.branches.base != "main" || in.Branch == nil || len(in.Branch.Files) != 1 || !in.Proofs[1] {
			t.Errorf("base = %q, branch = %+v, proofs = %v", f.branches.base, in.Branch, in.Proofs)
		}
		if in.Run.ID != "run-1" || in.Run.Phase != "planning" || in.Run.Track == "" {
			t.Errorf("run = %+v", in.Run)
		}
		want := []attribute.KeyValue{
			attribute.Int("harness.plan.problems", len(in.Problems)),
			attribute.Int("harness.view.bytes", len(pageBody)),
		}
		attrs := viewSpan(t, f)
		for _, kv := range want {
			if !slices.Contains(attrs, kv) {
				t.Errorf("attributes = %v, want %v", attrs, kv)
			}
		}
	})

	t.Run("charts and sketches", func(t *testing.T) {
		f := newFixture(t, trunkBranch)
		locked := filepath.Join(f.dir, "ui", "locked.excalidraw")
		write(t, locked, goodScene)
		if err := os.Chmod(locked, 0o000); err != nil {
			t.Fatal(err)
		}

		if _, err := f.handler.Handle(t.Context(), renderview.Command{Path: f.plan}); err != nil {
			t.Fatalf("Handle: %v", err)
		}

		in := f.renderer.input
		var states []domain.Diagram
		for _, d := range in.Doc.Diagrams {
			if d.Kind == domain.DiagramState {
				states = append(states, d)
			}
		}
		if len(states) != 3 || in.Charts[states[1].Line] != chartMark {
			t.Fatalf("states = %v, charts = %v, want the second converted", states, in.Charts)
		}
		if _, kept := in.Charts[states[2].Line]; kept {
			t.Error("a diagram with no flowchart form must keep its original block")
		}
		if string(in.Sketches["ui/good.excalidraw"]) != sceneSVG || string(in.Sketches["ui/bad.excalidraw"]) != sceneError {
			t.Errorf("sketches = %q", in.Sketches)
		}
		if _, ok := in.Sketches["ui/lost.excalidraw"]; ok {
			t.Error("a missing sketch file must leave no entry")
		}
		if got := string(in.Sketches["ui/locked.excalidraw"]); !strings.Contains(got, "permission denied") {
			t.Errorf("an unreadable sketch = %q, want its read error", got)
		}
	})

	t.Run("a branch read error gives no evidence and a page problem", func(t *testing.T) {
		f := newFixture(t, trunkBranch)
		f.branches.err = errors.New(branchError)

		_, err := f.handler.Handle(t.Context(), renderview.Command{Path: f.plan, Proofs: true})

		in := f.renderer.input
		if err != nil || in.Branch != nil {
			t.Fatalf("err = %v, branch = %+v, want nil and nil", err, in.Branch)
		}
		last := in.Problems[len(in.Problems)-1]
		if last.Rule != renderview.RuleBranchRead || !strings.Contains(last.Message, branchError) {
			t.Errorf("last problem = %+v", last)
		}
		if !slices.Contains(viewSpan(t, f), attribute.Int("harness.plan.problems", len(in.Problems))) {
			t.Error("the span must count the read problem")
		}
	})

	t.Run("a default branch error gives a page problem and no read", func(t *testing.T) {
		f := newFixture(t, func(context.Context) (string, error) { return "", errors.New(branchError) })
		text, _ := os.ReadFile(f.plan)
		write(t, f.plan, strings.Replace(string(text), ", based on `main`", "", 1))

		_, err := f.handler.Handle(t.Context(), renderview.Command{Path: f.plan})

		in := f.renderer.input
		last := in.Problems[len(in.Problems)-1]
		if err != nil || in.Branch != nil || f.branches.base != "" || last.Rule != renderview.RuleBranchRead {
			t.Errorf("err = %v, branch = %+v, base = %q, last = %+v", err, in.Branch, f.branches.base, last)
		}
	})

	t.Run("the default branch serves a plan with no base", func(t *testing.T) {
		f := newFixture(t, func(context.Context) (string, error) { return "trunk", nil })
		text, _ := os.ReadFile(f.plan)
		write(t, f.plan, strings.Replace(string(text), ", based on `main`", "", 1))

		_, err := f.handler.Handle(t.Context(), renderview.Command{Path: f.plan})

		if err != nil || f.branches.base != "trunk" {
			t.Errorf("err = %v, base = %q, want trunk", err, f.branches.base)
		}
	})

	t.Run("a proof read error gives no proof and a page problem", func(t *testing.T) {
		f := newFixture(t, trunkBranch)
		f.proofs.err = errors.New(proofError)
		f.proofs.passed = nil

		_, err := f.handler.Handle(t.Context(), renderview.Command{Path: f.plan, Proofs: true})

		in := f.renderer.input
		last := in.Problems[len(in.Problems)-1]
		if err != nil || len(in.Proofs) != 0 || last.Rule != renderview.RuleProofRead || !strings.Contains(last.Message, proofError) {
			t.Errorf("err = %v, proofs = %v, last = %+v", err, in.Proofs, last)
		}
	})

	t.Run("no proofs flag reads no proof", func(t *testing.T) {
		f := newFixture(t, trunkBranch)

		_, err := f.handler.Handle(t.Context(), renderview.Command{Path: f.plan})

		if err != nil || f.proofs.called || len(f.renderer.input.Proofs) != 0 {
			t.Errorf("err = %v, called = %v, proofs = %v", err, f.proofs.called, f.renderer.input.Proofs)
		}
	})

	t.Run("out replaces the page path", func(t *testing.T) {
		f := newFixture(t, trunkBranch)
		target := filepath.Join(t.TempDir(), "other.html")

		out, err := f.handler.Handle(t.Context(), renderview.Command{Path: f.plan, Out: target})

		if err != nil || out != target {
			t.Fatalf("out = %q, err = %v, want %q", out, err, target)
		}
	})

	t.Run("an unreadable plan fails and the span records the error", func(t *testing.T) {
		f := newFixture(t, trunkBranch)

		_, err := f.handler.Handle(t.Context(), renderview.Command{Path: filepath.Join(f.dir, "missing.md")})

		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("err = %v, want not exist", err)
		}
		for _, s := range f.spans.Ended() {
			if s.Name() == "plan.view" && s.Status().Description == "" {
				t.Error("the plan.view span has no error status")
			}
		}
	})
}
