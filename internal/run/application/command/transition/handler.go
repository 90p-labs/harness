package transition

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/run/domain"
)

const (
	spanTransition = "run.transition"
	byReasonFormat = "approved by %s: %s"
)

const (
	attrRunID     = attribute.Key("harness.run.id")
	attrEvent     = attribute.Key("harness.event")
	attrPhaseFrom = attribute.Key("harness.phase.from")
	attrPhaseTo   = attribute.Key("harness.phase.to")
)

var gatedEvents = map[domain.Event]bool{
	domain.EventSubmitPlan:  true,
	domain.EventAmend:       true,
	domain.EventApprovePlan: true,
}

type Command struct {
	Key     domain.RunKey
	Event   domain.Event
	Reason  string
	By      string
	PRURL   string
	RunID   string
	Version int
}

type Flow struct {
	repo      domain.RunRepository
	hooks     domain.TransitionHooks
	gate      domain.PlanGate
	approvals domain.ApprovalRepository
	policy    domain.Policy
	clock     func() time.Time
	tracer    trace.Tracer
}

func NewFlow(repo domain.RunRepository, hooks domain.TransitionHooks, gate domain.PlanGate, approvals domain.ApprovalRepository, policy domain.Policy, clock func() time.Time, tracer trace.Tracer) Flow {
	return Flow{repo: repo, hooks: hooks, gate: gate, approvals: approvals, policy: policy, clock: clock, tracer: tracer}
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
	if _, err := run.Apply(cmd, f.policy); err != nil {
		return err
	}
	policy := f.policy
	if gatedEvents[cmd.Event] {
		submission, err := f.gate.Submit(ctx, run)
		if err != nil {
			return err
		}
		cmd.Track = submission.Track
		if cmd.Event == domain.EventSubmitPlan && submission.ApprovalWaived {
			policy.ApprovalNeeded = false
		}
		if cmd.Event == domain.EventApprovePlan && submission.QuestionsOpen {
			return domain.ErrOpenQuestions
		}
	}
	if cmd.Event == domain.EventApprovePlan {
		if cmd, err = f.approved(ctx, run, cmd); err != nil {
			return err
		}
	}
	if err := f.hooks.Before(ctx, run, cmd); err != nil {
		return err
	}
	next, err := run.Apply(cmd, policy)
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

func (f Flow) approved(ctx context.Context, run domain.Run, cmd domain.Command) (domain.Command, error) {
	if cmd.By != "" {
		cmd.Reason = fmt.Sprintf(byReasonFormat, cmd.By, cmd.Reason)
		return cmd, nil
	}
	hash, err := f.gate.Hash(run)
	if err != nil {
		return cmd, err
	}
	recorded, err := f.approvals.Has(ctx, run.ID, hash)
	if err != nil {
		return cmd, err
	}
	if !recorded {
		return cmd, domain.ErrApprovalNotRecorded
	}
	return cmd, nil
}

type Handler struct {
	repo domain.RunRepository
	flow Flow
}

func NewHandler(repo domain.RunRepository, flow Flow) Handler {
	return Handler{repo: repo, flow: flow}
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	run, err := h.load(ctx, cmd)
	if err != nil {
		return err
	}
	return h.flow.Move(ctx, run, domain.Command{Event: cmd.Event, Reason: cmd.Reason, By: cmd.By, PRURL: cmd.PRURL})
}

func (h Handler) load(ctx context.Context, cmd Command) (domain.Run, error) {
	if cmd.RunID == "" {
		return h.repo.Active(ctx, cmd.Key)
	}
	run, err := h.repo.Get(ctx, cmd.RunID)
	if err != nil {
		return domain.Run{}, err
	}
	if run.Version != cmd.Version {
		return domain.Run{}, fmt.Errorf("%w: %s at version %d, want %d", domain.ErrStaleRun, run.ID, run.Version, cmd.Version)
	}
	return run, nil
}
