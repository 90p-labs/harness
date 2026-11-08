package transition

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/run/domain"
)

const spanTransition = "run.transition"

const (
	attrRunID     = attribute.Key("harness.run.id")
	attrEvent     = attribute.Key("harness.event")
	attrPhaseFrom = attribute.Key("harness.phase.from")
	attrPhaseTo   = attribute.Key("harness.phase.to")
)

type Command struct {
	Key    domain.RunKey
	Event  domain.Event
	Reason string
}

type Flow struct {
	repo   domain.RunRepository
	hooks  domain.TransitionHooks
	policy domain.Policy
	clock  func() time.Time
	tracer trace.Tracer
}

func NewFlow(repo domain.RunRepository, hooks domain.TransitionHooks, policy domain.Policy, clock func() time.Time, tracer trace.Tracer) Flow {
	return Flow{repo: repo, hooks: hooks, policy: policy, clock: clock, tracer: tracer}
}

func (f Flow) Move(ctx context.Context, run domain.Run, cmd domain.Command) (err error) {
	ctx, span := f.tracer.Start(ctx, spanTransition, trace.WithAttributes(
		attrRunID.String(run.ID),
		attrEvent.String(string(cmd.Event)),
		attrPhaseFrom.String(string(run.Phase)),
	))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	cmd.At = f.clock()
	if err := f.hooks.Before(ctx, run, cmd); err != nil {
		return err
	}
	next, err := run.Apply(cmd, f.policy)
	if err != nil {
		return err
	}
	if _, err := f.repo.Save(ctx, next); err != nil {
		return err
	}
	f.hooks.After(ctx, run, next)
	span.SetAttributes(attrPhaseTo.String(string(next.Phase)))
	return nil
}

type Handler struct {
	repo domain.RunRepository
	flow Flow
}

func NewHandler(repo domain.RunRepository, flow Flow) Handler {
	return Handler{repo: repo, flow: flow}
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	run, err := h.repo.Active(ctx, cmd.Key)
	if err != nil {
		return err
	}
	return h.flow.Move(ctx, run, domain.Command{Event: cmd.Event, Reason: cmd.Reason})
}
