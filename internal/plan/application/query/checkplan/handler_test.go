package checkplan_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/plan/application/query/checkplan"
	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	validPlan         = "../../../domain/testdata/valid.md"
	unknownBlock      = "\n```mermaid\nflowchart TD\n    a --> b\n```\n"
	archivedName      = "plan-1.md"
	regressionSection = "\n## Regression impact\n\nThe change touches the work of plan-1.md.\n"
	brokenCriteria    = "# Plan\n\n| # | Criterion | Proof |\n|---|---|---|\n| 2 | x | y |\n"
	planFileName      = "plan.md"
)

func newHandler(t *testing.T) (checkplan.Handler, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	return checkplan.NewHandler(nil, provider.Tracer("test")), recorder
}

func writePlan(t *testing.T, dir string, body ...[]byte) string {
	t.Helper()
	path := filepath.Join(dir, planFileName)
	if err := os.WriteFile(path, slices.Concat(body...), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCheckPlanSpan(t *testing.T) {
	handler, recorder := newHandler(t)

	report, err := handler.Handle(t.Context(), checkplan.Query{Path: validPlan})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(report.Problems) != 0 || report.Track != shareddomain.TrackFull {
		t.Fatalf("report = %+v, want no problem and the full track", report)
	}
	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "plan.check" {
		t.Fatalf("spans = %v, want one plan.check", spans)
	}
	want := []attribute.KeyValue{
		attribute.Int("harness.plan.problems", 0),
		attribute.String("harness.track", string(shareddomain.TrackFull)),
	}
	for _, kv := range want {
		if !slices.Contains(spans[0].Attributes(), kv) {
			t.Errorf("attributes = %v, want %v", spans[0].Attributes(), kv)
		}
	}
}

func TestCheckPlanSketches(t *testing.T) {
	valid, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := newHandler(t)
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "ui"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ui", "x.excalidraw"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	links := "\n## Notes\n\n![Found](ui/x.excalidraw)\n![Lost](ui/y.excalidraw)\n"

	report, err := handler.Handle(t.Context(), checkplan.Query{Path: writePlan(t, dir, valid, []byte(links))})

	if err != nil {
		t.Fatalf("Handle: %v", err)
	}
	var refused []domain.Problem
	for _, p := range report.Problems {
		if p.Rule == domain.RuleDiagramRequired {
			refused = append(refused, p)
		}
	}
	if len(refused) != 1 || !strings.Contains(refused[0].Message, "ui/y.excalidraw") {
		t.Fatalf("diagram-required = %v, want one problem for ui/y.excalidraw", refused)
	}
}

func TestCheckPlanSketchWalk(t *testing.T) {
	valid, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := newHandler(t)
	dir := t.TempDir()
	for _, folder := range []string{"ui", ".cache", "node_modules"} {
		if err := os.MkdirAll(filepath.Join(dir, folder), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, folder, "x.excalidraw"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	links := "\n## Notes\n\n![Ui](ui/x.excalidraw)\n![Cache](.cache/x.excalidraw)\n![Modules](node_modules/x.excalidraw)\n"

	report, err := handler.Handle(t.Context(), checkplan.Query{Path: writePlan(t, dir, valid, []byte(links))})

	if err != nil {
		t.Fatalf("an unreadable folder must not stop the check: %v", err)
	}
	var refused []string
	for _, p := range report.Problems {
		if p.Rule == domain.RuleDiagramRequired {
			refused = append(refused, p.Message)
		}
	}
	if len(refused) != 2 || !strings.Contains(refused[0], ".cache/") || !strings.Contains(refused[1], "node_modules/") {
		t.Fatalf("diagram-required = %v, want the .cache and node_modules links refused", refused)
	}
}

func TestCheckPlanReport(t *testing.T) {
	valid, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := newHandler(t)

	t.Run("a diagram of an unchecked kind is a note", func(t *testing.T) {
		report, err := handler.Handle(t.Context(), checkplan.Query{Path: writePlan(t, t.TempDir(), valid, []byte(unknownBlock))})

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if len(report.Notes) != 1 || !strings.HasPrefix(report.Notes[0], "plan.md:") || !strings.HasSuffix(report.Notes[0], ": note: diagram kind flowchart is not checked") {
			t.Errorf("notes = %q, want one flowchart note", report.Notes)
		}
	})

	t.Run("the archived plans beside the file feed the regression rule", func(t *testing.T) {
		withArchive, withoutArchive := t.TempDir(), t.TempDir()
		withPath := writePlan(t, withArchive, valid, []byte(regressionSection))
		withoutPath := writePlan(t, withoutArchive, valid, []byte(regressionSection))
		if err := os.WriteFile(filepath.Join(withArchive, archivedName), nil, 0o600); err != nil {
			t.Fatal(err)
		}

		hasRegression := func(path string) bool {
			report, err := handler.Handle(t.Context(), checkplan.Query{Path: path, LiveRound: 1})
			if err != nil {
				t.Fatalf("Handle: %v", err)
			}
			return slices.ContainsFunc(report.Problems, func(p domain.Problem) bool { return p.Rule == domain.RuleRegressionImpact })
		}

		if hasRegression(withPath) || !hasRegression(withoutPath) {
			t.Errorf("problem with archive = %v, without = %v, want false and true", hasRegression(withPath), hasRegression(withoutPath))
		}
	})

	t.Run("the problems are sorted by line", func(t *testing.T) {
		report, err := handler.Handle(t.Context(), checkplan.Query{Path: writePlan(t, t.TempDir(), []byte(brokenCriteria))})

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if len(report.Problems) < 2 {
			t.Fatalf("problems = %v, want at least 2", report.Problems)
		}
		if !slices.IsSortedFunc(report.Problems, func(a, b domain.Problem) int { return a.Line - b.Line }) {
			t.Errorf("problems are not sorted by line: %v", report.Problems)
		}
	})

	t.Run("the report carries the parsed document", func(t *testing.T) {
		report, err := handler.Handle(t.Context(), checkplan.Query{Path: writePlan(t, t.TempDir(), valid)})

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if report.Doc.Base() != "main" || len(report.Doc.Tasks) != 2 {
			t.Errorf("Doc base = %q with %d tasks, want main and 2", report.Doc.Base(), len(report.Doc.Tasks))
		}
	})

	t.Run("a missing file returns the read error", func(t *testing.T) {
		_, err := handler.Handle(t.Context(), checkplan.Query{Path: filepath.Join(t.TempDir(), "missing.md")})

		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("err = %v, want not exist", err)
		}
	})
}

func TestProgramKind(t *testing.T) {
	program, err := os.ReadFile("../../../domain/testdata/program.md")
	if err != nil {
		t.Fatal(err)
	}
	valid, err := os.ReadFile(validPlan)
	if err != nil {
		t.Fatal(err)
	}
	handler, _ := newHandler(t)

	t.Run("a program has runs, no document and no problem", func(t *testing.T) {
		report, err := handler.Handle(t.Context(), checkplan.Query{Path: writePlan(t, t.TempDir(), program)})

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if len(report.Problems) != 0 || report.Program == nil || len(report.Program.Runs) != 8 {
			t.Fatalf("report = %+v, want no problem and 8 runs", report)
		}
		if report.Track != "" || len(report.Doc.Lines) != 0 {
			t.Errorf("track = %q, doc = %+v, want both empty", report.Track, report.Doc)
		}
	})

	t.Run("a broken program gives its rule", func(t *testing.T) {
		broken := slices.Concat(program, []byte("\nA broken \u2014 line.\n"))

		report, err := handler.Handle(t.Context(), checkplan.Query{Path: writePlan(t, t.TempDir(), broken)})

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if len(report.Problems) != 1 || report.Problems[0].Rule != domain.RuleEmDash {
			t.Fatalf("problems = %v, want one %s", report.Problems, domain.RuleEmDash)
		}
	})

	t.Run("a single-run plan has no program", func(t *testing.T) {
		report, err := handler.Handle(t.Context(), checkplan.Query{Path: writePlan(t, t.TempDir(), valid)})

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if report.Program != nil || len(report.Problems) != 0 || report.Track != shareddomain.TrackFull {
			t.Errorf("report = %+v, want no program, no problem and the full track", report)
		}
	})
}
