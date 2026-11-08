package renderchart

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jorge07/harness/internal/deploy/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type Query struct {
	Dir   string
	Chart string
}

type Handler struct {
	bus shareddomain.Dispatcher
}

func NewHandler(bus shareddomain.Dispatcher) Handler {
	return Handler{bus: bus}
}

func (h Handler) Handle(ctx context.Context, q Query) ([]byte, error) {
	payload, err := json.Marshal(shareddomain.ChartRenderPayload{Chart: q.Chart})
	if err != nil {
		return nil, err
	}
	result, err := h.bus.Dispatch(ctx, shareddomain.Envelope{
		Version: shareddomain.EnvelopeVersion,
		Event:   shareddomain.EventChartRender,
		Cwd:     q.Dir,
		Payload: payload,
	})
	if err != nil {
		return nil, err
	}
	failures := make([]error, len(result.Failures))
	for i, failure := range result.Failures {
		failures[i] = failure.Err
	}
	if err := errors.Join(failures...); err != nil {
		return nil, fmt.Errorf("render %s: %w", q.Chart, err)
	}
	if result.Verdict.Kind == shareddomain.VerdictDeny {
		return nil, fmt.Errorf("render %s: %s", q.Chart, result.Verdict.Reason)
	}
	if result.Verdict.Context == "" {
		return nil, fmt.Errorf("%w: %s", domain.ErrNoRenderer, q.Chart)
	}
	return []byte(result.Verdict.Context), nil
}
