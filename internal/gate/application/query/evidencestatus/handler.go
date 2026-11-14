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

// Record is the result of a gate for the current files and the tail of its output.
type Record struct {
	Result domain.Result
	Tail   string
}

func (h Handler) Handle(ctx context.Context, q Query) (map[string]domain.Result, error) {
	records, err := h.Records(ctx, q)
	if err != nil {
		return nil, err
	}
	statuses := make(map[string]domain.Result, len(records))
	for id, record := range records {
		statuses[id] = record.Result
	}
	return statuses, nil
}

// Records gives the result and the tail of each gate that has a record for the current files.
func (h Handler) Records(ctx context.Context, q Query) (records map[string]Record, err error) {
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
	records = make(map[string]Record, len(q.Gates))
	for i, gate := range q.Gates {
		var kept domain.Evidence
		var ok bool
		if kept, ok, err = h.evidence.Find(ctx, q.Repo, gate.ID, hashes[i]); err != nil {
			return nil, err
		}
		if ok {
			records[gate.ID] = Record{Result: kept.Result, Tail: kept.Tail}
		}
	}
	return records, nil
}
