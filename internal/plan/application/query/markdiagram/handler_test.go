package markdiagram_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jorge07/harness/internal/plan/application/query/markdiagram"
	"github.com/jorge07/harness/internal/plan/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	markedFlowchart = "flowchart TD\n    a[start] --> b[end] +\n"
	markedState     = "stateDiagram-v2\n    [*] --> Ready\n    Ready --> Done +\n"
	markedClass     = "classDiagram\n    class Pong +\n"
	unmarkedState   = "stateDiagram-v2\n    [*] --> Ready\n"
	kindFlowchart   = "flowchart"
	parserReason    = "parser says no"
)

var errParser = errors.New(parserReason)

type fakeParser struct {
	err   error
	kinds []string
}

func (f *fakeParser) Parse(kind, _ string) error {
	f.kinds = append(f.kinds, kind)
	return f.err
}

func newHandler(parser domain.DiagramParser) (markdiagram.Handler, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	return markdiagram.NewHandler(parser, provider.Tracer("test")), recorder
}

func TestMarkDiagramQuery(t *testing.T) {
	t.Run("a flowchart is marked and skips the parser", func(t *testing.T) {
		parser := &fakeParser{err: errParser}
		handler, recorder := newHandler(parser)

		body, err := handler.Handle(t.Context(), markedFlowchart)

		if err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if !strings.Contains(body, "classDef new") || !strings.Contains(body, `|"(new)"|`) {
			t.Errorf("body = %q, want the new classDef and a marked edge", body)
		}
		if strings.Contains(body, "\n\n") || strings.HasSuffix(body, "\n") {
			t.Errorf("body = %q, want no blank line", body)
		}
		if len(parser.kinds) != 0 {
			t.Errorf("parsed kinds = %v, want none", parser.kinds)
		}
		spans := recorder.Ended()
		if len(spans) != 1 || spans[0].Name() != "diagram.mark" {
			t.Fatalf("spans = %v, want one diagram.mark", spans)
		}
		want := attribute.String("harness.diagram.kind", kindFlowchart)
		if !slices.Contains(spans[0].Attributes(), want) {
			t.Errorf("attributes = %v, want %v", spans[0].Attributes(), want)
		}
	})

	t.Run("a state and a class diagram go through the parser", func(t *testing.T) {
		for _, tc := range []struct{ input, kind string }{
			{markedState, domain.DiagramState},
			{markedClass, domain.DiagramClass},
		} {
			parser := &fakeParser{}
			handler, recorder := newHandler(parser)

			if _, err := handler.Handle(t.Context(), tc.input); err != nil {
				t.Fatalf("Handle %s: %v", tc.kind, err)
			}
			if !slices.Equal(parser.kinds, []string{tc.kind}) {
				t.Errorf("parsed kinds = %v, want [%s]", parser.kinds, tc.kind)
			}
			want := attribute.String("harness.diagram.kind", tc.kind)
			if !slices.Contains(recorder.Ended()[0].Attributes(), want) {
				t.Errorf("attributes = %v, want %v", recorder.Ended()[0].Attributes(), want)
			}
		}
	})

	t.Run("a parser refusal is a refused diagram", func(t *testing.T) {
		handler, recorder := newHandler(&fakeParser{err: errParser})

		body, err := handler.Handle(t.Context(), markedState)

		if !errors.Is(err, shareddomain.ErrDiagramRefused) || !strings.Contains(err.Error(), parserReason) {
			t.Fatalf("err = %v, want a refused diagram with %q", err, parserReason)
		}
		if body != "" {
			t.Errorf("body = %q, want empty", body)
		}
		spans := recorder.Ended()
		if len(spans) != 1 {
			t.Fatalf("spans = %d, want 1", len(spans))
		}
		want := attribute.String("harness.diagram.kind", domain.DiagramState)
		if !slices.Contains(spans[0].Attributes(), want) {
			t.Errorf("attributes = %v, want %v", spans[0].Attributes(), want)
		}
	})

	t.Run("an unmarked diagram is refused before the parser", func(t *testing.T) {
		parser := &fakeParser{}
		handler, _ := newHandler(parser)

		_, err := handler.Handle(t.Context(), unmarkedState)

		if !errors.Is(err, shareddomain.ErrDiagramRefused) {
			t.Fatalf("err = %v, want a refused diagram", err)
		}
		if len(parser.kinds) != 0 {
			t.Errorf("parsed kinds = %v, want none", parser.kinds)
		}
	})
}
