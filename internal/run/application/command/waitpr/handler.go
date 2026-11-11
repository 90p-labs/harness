package waitpr

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/run/domain"
)

const spanWaitPR = "run.wait_pr"

const (
	attrRunID = attribute.Key("harness.run.id")
	attrPhase = attribute.Key("harness.phase.from")
)

type Command struct {
	Key   domain.RunKey
	PRURL string
}

type Handler struct {
	repo   domain.RunRepository
	tracer trace.Tracer
}

func NewHandler(repo domain.RunRepository, tracer trace.Tracer) Handler {
	return Handler{repo: repo, tracer: tracer}
}

func (h Handler) Handle(ctx context.Context, cmd Command) (err error) {
	ctx, span := h.tracer.Start(ctx, spanWaitPR)
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	run, err := h.repo.Active(ctx, cmd.Key)
	if err != nil {
		return err
	}
	span.SetAttributes(attrRunID.String(run.ID), attrPhase.String(string(run.Phase)))
	waiting, err := run.WaitOnPR(cmd.PRURL)
	if err != nil {
		return err
	}
	_, err = h.repo.Save(ctx, waiting)
	return err
}
