package recordapproval

import (
	"context"
	"fmt"
	"time"

	"github.com/90p-labs/harness/internal/run/domain"
)

type Command struct {
	RunID string
}

type Handler struct {
	repo      domain.RunRepository
	gate      domain.PlanGate
	approvals domain.ApprovalRepository
	now       func() time.Time
}

func NewHandler(repo domain.RunRepository, gate domain.PlanGate, approvals domain.ApprovalRepository, now func() time.Time) Handler {
	return Handler{repo: repo, gate: gate, approvals: approvals, now: now}
}

func (h Handler) Handle(ctx context.Context, c Command) error {
	run, err := h.repo.Get(ctx, c.RunID)
	if err != nil {
		return err
	}
	if run.Phase != domain.PhaseAwaitingApproval {
		return fmt.Errorf("%w: record an approval in phase %q", domain.ErrIllegalTransition, run.Phase)
	}
	hash, err := h.gate.Hash(run)
	if err != nil {
		return err
	}
	return h.approvals.Save(ctx, run.ID, hash, h.now())
}
