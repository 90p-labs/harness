package decidestop

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/run/domain"
)

const spanDecideStop = "run.decide_stop"

const attrBlock = attribute.Key("harness.stop.block")

type Command struct {
	Key          domain.RunKey
	AgentRunning bool
}

type Result = domain.StopVerdict

type Handler struct {
	repo   domain.RunRepository
	policy domain.Policy
	tracer trace.Tracer
}

func NewHandler(repo domain.RunRepository, policy domain.Policy, tracer trace.Tracer) Handler {
	return Handler{repo: repo, policy: policy, tracer: tracer}
}

func (h Handler) Handle(ctx context.Context, cmd Command) (result Result, err error) {
	ctx, span := h.tracer.Start(ctx, spanDecideStop)
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.SetAttributes(attrBlock.Bool(result.Block))
	}()

	run, err := h.repo.Active(ctx, cmd.Key)
	if errors.Is(err, domain.ErrRunNotFound) {
		return Result{}, nil
	}
	if err != nil {
		return Result{}, err
	}
	verdict, next := domain.DecideStop(run, h.policy, cmd.AgentRunning)
	if !verdict.Block {
		return verdict, nil
	}
	if _, err := h.repo.Save(ctx, next); err != nil {
		return Result{}, err
	}
	return verdict, nil
}
