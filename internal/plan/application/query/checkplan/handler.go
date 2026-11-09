package checkplan

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/plan/domain"
)

const (
	spanName         = "plan.check"
	attrProblems     = "harness.plan.problems"
	attrTrack        = "harness.track"
	archivedPattern  = "plan-*.md"
	noteFormat       = "%s:%d: note: diagram kind %s is not checked"
	hiddenPrefix     = "."
	dependencyFolder = "node_modules"
)

type Query struct {
	Path      string
	LiveRound int
}

type Handler struct {
	parser domain.DiagramParser
	tracer trace.Tracer
}

func NewHandler(parser domain.DiagramParser, tracer trace.Tracer) Handler {
	return Handler{parser: parser, tracer: tracer}
}

func (h Handler) Handle(ctx context.Context, q Query) (report domain.Report, err error) {
	_, span := h.tracer.Start(ctx, spanName)
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.SetAttributes(
			attribute.Int(attrProblems, len(report.Problems)),
			attribute.String(attrTrack, string(report.Track)),
		)
		span.End()
	}()

	text, err := os.ReadFile(q.Path)
	if err != nil {
		return domain.Report{}, err
	}
	if domain.IsProgram(string(text)) {
		program, problems := domain.ParseProgram(string(text))
		problems = append(problems, domain.CheckProgram(program)...)
		domain.SortProblems(problems)
		return domain.Report{Problems: problems, Program: &program}, nil
	}
	archived, err := archivedPlans(q.Path)
	if err != nil {
		return domain.Report{}, err
	}
	sketches, err := sketchFiles(filepath.Dir(q.Path))
	if err != nil {
		return domain.Report{}, err
	}
	doc, problems := domain.Parse(string(text))
	problems = append(problems, domain.Check(doc, domain.CheckInput{
		LiveRound: q.LiveRound, ArchivedPlans: archived, Diagrams: h.parser, Sketches: sketches,
	})...)
	domain.SortProblems(problems)
	return domain.Report{Problems: problems, Notes: notes(doc), Track: domain.Classify(doc), Doc: doc}, nil
}

func sketchFiles(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if p != root && d.IsDir() && (err != nil || strings.HasPrefix(d.Name(), hiddenPrefix) || d.Name() == dependencyFolder) {
			return fs.SkipDir
		}
		if err != nil || d.IsDir() || filepath.Ext(p) != domain.SketchExt {
			return err
		}
		rel, err := filepath.Rel(root, p)
		found = append(found, filepath.ToSlash(rel))
		return err
	})
	return found, err
}

func archivedPlans(path string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), archivedPattern))
	if err != nil {
		return nil, err
	}
	names := make([]string, len(matches))
	for i, m := range matches {
		names[i] = filepath.Base(m)
	}
	return names, nil
}

func notes(doc domain.Document) []string {
	var out []string
	for _, d := range doc.Diagrams {
		if !d.Checked() {
			out = append(out, fmt.Sprintf(noteFormat, domain.PlanFile, d.Line, d.Kind))
		}
	}
	return out
}
