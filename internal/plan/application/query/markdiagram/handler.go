package markdiagram

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/plan/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	spanName = "diagram.mark"
	attrKind = "harness.diagram.kind"
)

type Handler struct {
	parser domain.DiagramParser
	tracer trace.Tracer
}

func NewHandler(parser domain.DiagramParser, tracer trace.Tracer) Handler {
	return Handler{parser: parser, tracer: tracer}
}

func (h Handler) Handle(ctx context.Context, input string) (body string, err error) {
	_, span := h.tracer.Start(ctx, spanName)
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

	body, err = shareddomain.MarkDiagram(input)
	if err != nil {
		return "", err
	}
	diagram := domain.Diagram{Kind: shareddomain.DiagramKind(body), Body: body}
	span.SetAttributes(attribute.String(attrKind, diagram.Kind))
	if diagram.Checked() {
		if err := h.parser.Parse(diagram.Kind, diagram.Body); err != nil {
			return "", fmt.Errorf("%w: %w", shareddomain.ErrDiagramRefused, err)
		}
	}
	return body, nil
}
