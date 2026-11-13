package learn

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	spanLearn = "run.learn"
	attrRunID = attribute.Key("harness.run.id")
)

type Query struct {
	Key   domain.RunKey
	RunID string
}

type Learn struct {
	ID         string
	Candidates []domain.Candidate
}

type Handler struct {
	repo     domain.RunRepository
	log      domain.TransitionLog
	findings domain.FindingsReader
	tracer   trace.Tracer
}

func NewHandler(repo domain.RunRepository, log domain.TransitionLog, findings domain.FindingsReader, tracer trace.Tracer) Handler {
	return Handler{repo: repo, log: log, findings: findings, tracer: tracer}
}

func (h Handler) Handle(ctx context.Context, query Query) (result Learn, err error) {
	ctx, span := h.tracer.Start(ctx, spanLearn)
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	var run domain.Run
	if query.RunID != "" {
		run, err = h.repo.Get(ctx, query.RunID)
	} else {
		run, err = h.repo.Active(ctx, query.Key)
	}
	if err != nil {
		return Learn{}, err
	}
	span.SetAttributes(attrRunID.String(run.ID))
	rows, err := h.log.Transitions(ctx, run.ID)
	if err != nil {
		return Learn{}, err
	}
	found, err := h.findings.Read(ctx, run)
	if err != nil && !errors.Is(err, domain.ErrNoFindingsFile) {
		return Learn{}, err
	}
	return Learn{ID: run.ID, Candidates: domain.ConventionCandidates(found, rows)}, nil
}
