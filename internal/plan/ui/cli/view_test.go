package cli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/jorge07/harness/internal/plan/application/command/renderview"
	"github.com/jorge07/harness/internal/plan/application/query/checkplan"
	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/plan/ui/cli"
)

const (
	subView    = "view"
	flagOut    = "--out"
	pageName   = "plan.html"
	pageBody   = "<html>page</html>"
	runIDValue = "20261108-081430-add-the-login-page"
)

type pageRenderer struct{ inputs []domain.ViewInput }

func (r *pageRenderer) Render(in domain.ViewInput) ([]byte, error) {
	r.inputs = append(r.inputs, in)
	return []byte(pageBody), nil
}

type noBranch struct{}

func (noBranch) Read(context.Context, string, string, []string) (domain.Branch, error) {
	return domain.Branch{}, errors.New("no branch")
}

type noProofs struct{}

func (noProofs) Passed(context.Context) (map[int]bool, error) { return nil, errors.New("no proofs") }

type plainSketches struct{}

func (plainSketches) RenderSVG([]byte) ([]byte, error) { return []byte("<svg/>"), nil }

type plainCharts struct{}

func (plainCharts) StateChart(body string) (string, error) { return body, nil }

func viewConnect(renderer domain.ViewRenderer, defaultView func(context.Context) (renderview.Command, error)) cli.Connect {
	return func(context.Context) (cli.Handlers, func() error, error) {
		tracer := noop.NewTracerProvider().Tracer("test")
		check := checkplan.NewHandler(nil, tracer)
		view := renderview.NewHandler(renderview.Dependencies{
			Check: func(ctx context.Context, path string, liveRound int) (domain.Report, error) {
				return check.Handle(ctx, checkplan.Query{Path: path, LiveRound: liveRound})
			},
			Branches:      noBranch{},
			Proofs:        noProofs{},
			Charts:        plainCharts{},
			Sketches:      plainSketches{},
			Renderer:      renderer,
			DefaultBranch: func(context.Context) (string, error) { return "main", nil },
			Tracer:        tracer,
		})
		return cli.Handlers{View: view, DefaultView: defaultView}, func() error { return nil }, nil
	}
}

func TestPlanViewCommand(t *testing.T) {
	failDefault := func(context.Context) (renderview.Command, error) {
		t.Error("DefaultView called")
		return renderview.Command{}, nil
	}
	copyPlan := func(t *testing.T, body string) string {
		t.Helper()
		path := filepath.Join(t.TempDir(), "plan.md")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	valid, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("a file writes the page beside the plan and prints the path", func(t *testing.T) {
		plan := copyPlan(t, string(valid))
		renderer := &pageRenderer{}

		out, err := execute(t, viewConnect(renderer, failDefault), subView, flagFile, plan)

		page := filepath.Join(filepath.Dir(plan), pageName)
		if err != nil || strings.TrimSpace(out) != page {
			t.Fatalf("err = %v, out = %q, want %q", err, out, page)
		}
		if got, _ := os.ReadFile(page); string(got) != pageBody {
			t.Errorf("page = %q, want %q", got, pageBody)
		}
		if len(renderer.inputs) != 1 || renderer.inputs[0].Branch != nil || len(renderer.inputs[0].Proofs) != 0 {
			t.Errorf("inputs = %+v, want no branch and no proof", renderer.inputs)
		}
	})

	t.Run("a plan with problems still writes the page", func(t *testing.T) {
		plan := copyPlan(t, brokenPlan)
		renderer := &pageRenderer{}

		out, err := execute(t, viewConnect(renderer, failDefault), subView, flagFile, plan)

		if err != nil || strings.TrimSpace(out) != filepath.Join(filepath.Dir(plan), pageName) {
			t.Fatalf("err = %v, out = %q, want the page path", err, out)
		}
		if len(renderer.inputs) != 1 || len(renderer.inputs[0].Problems) == 0 {
			t.Errorf("inputs = %+v, want the plan problems", renderer.inputs)
		}
	})

	t.Run("out replaces the page path", func(t *testing.T) {
		plan := copyPlan(t, string(valid))
		target := filepath.Join(t.TempDir(), "elsewhere.html")

		out, err := execute(t, viewConnect(&pageRenderer{}, failDefault), subView, flagFile, plan, flagOut, target)

		if err != nil || strings.TrimSpace(out) != target {
			t.Fatalf("err = %v, out = %q, want %q", err, out, target)
		}
		if _, err := os.Stat(target); err != nil {
			t.Error(err)
		}
	})

	t.Run("no file uses the default view in each phase", func(t *testing.T) {
		for _, phase := range []string{"planning", "awaiting_approval", "working", "reviewing", "done"} {
			plan := copyPlan(t, string(valid))
			renderer := &pageRenderer{}
			defaultView := func(context.Context) (renderview.Command, error) {
				return renderview.Command{Path: plan, RunID: runIDValue, Phase: phase, Proofs: true}, nil
			}

			out, err := execute(t, viewConnect(renderer, defaultView), subView)

			if err != nil || strings.TrimSpace(out) != filepath.Join(filepath.Dir(plan), pageName) {
				t.Fatalf("%s: err = %v, out = %q", phase, err, out)
			}
			run := renderer.inputs[0].Run
			if run.ID != runIDValue || run.Phase != phase {
				t.Errorf("%s: run = %+v", phase, run)
			}
		}
	})

	t.Run("no file and no run is the usage error", func(t *testing.T) {
		noRun := func(context.Context) (renderview.Command, error) { return renderview.Command{}, cli.ErrNoPlanFile }

		_, err := execute(t, viewConnect(&pageRenderer{}, noRun), subView)

		if !errors.Is(err, cli.ErrNoPlanFile) {
			t.Fatalf("err = %v, want ErrNoPlanFile", err)
		}
	})

	t.Run("an unreadable plan fails", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.md")

		_, err := execute(t, viewConnect(&pageRenderer{}, failDefault), subView, flagFile, missing)

		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("err = %v, want not exist", err)
		}
	})
}
