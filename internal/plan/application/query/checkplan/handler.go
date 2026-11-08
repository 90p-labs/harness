package checkplan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/plan/domain"
)

const (
	spanName        = "plan.check"
	attrProblems    = "harness.plan.problems"
	attrTrack       = "harness.track"
	archivedPattern = "plan-*.md"
	noteFormat      = "%s:%d: note: diagram kind %s is not checked"
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
	archived, err := archivedPlans(q.Path)
	if err != nil {
		return domain.Report{}, err
	}
	doc, problems := domain.Parse(string(text))
	problems = append(problems, domain.Check(doc, domain.CheckInput{
		LiveRound: q.LiveRound, ArchivedPlans: archived, Diagrams: h.parser,
	})...)
	slices.SortStableFunc(problems, func(a, b domain.Problem) int {
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(string(a.Rule), string(b.Rule))
	})
	return domain.Report{Problems: problems, Notes: notes(doc), Track: domain.Classify(doc)}, nil
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
