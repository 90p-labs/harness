package evidencestatus

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/gate/domain"
)

const spanEvidenceStatus = "gate.evidence_status"

type Query struct {
	Repo  string
	Dir   string
	Gates []domain.Gate
}

type Handler struct {
	hasher   domain.BatchHasher
	evidence domain.EvidenceRepository
	tracer   trace.Tracer
}

func NewHandler(hasher domain.BatchHasher, evidence domain.EvidenceRepository, tracer trace.Tracer) Handler {
	return Handler{hasher: hasher, evidence: evidence, tracer: tracer}
}

func (h Handler) Handle(ctx context.Context, q Query) (statuses map[string]domain.Result, err error) {
	ctx, span := h.tracer.Start(ctx, spanEvidenceStatus)
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	hashes, err := h.hasher.InputHashes(ctx, q.Dir, q.Gates)
	if err != nil {
		return nil, err
	}
	if len(hashes) != len(q.Gates) {
		return nil, fmt.Errorf("hasher returned %d hashes for %d gates", len(hashes), len(q.Gates))
	}
	statuses = make(map[string]domain.Result, len(q.Gates))
	for i, gate := range q.Gates {
		record, ok, err := h.evidence.Find(ctx, q.Repo, gate.ID, hashes[i])
		if err != nil {
			return nil, err
		}
		if ok {
			statuses[gate.ID] = record.Result
		}
	}
	return statuses, nil
}
