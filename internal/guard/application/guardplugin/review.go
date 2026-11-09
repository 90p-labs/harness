package guardplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"go.opentelemetry.io/otel/metric"

	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	nameReviewRecorder = namePrefix + "review-recorder"
	reviewRecordFailed = "the review model was not recorded: "
)

type reviewRecorder struct {
	deps Deps
}

func (reviewRecorder) Name() string { return nameReviewRecorder }

func (reviewRecorder) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventToolAfter}
}

func (p reviewRecorder) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	if envelope.Run.Phase != shareddomain.PhaseReviewing {
		return allow, nil
	}
	var payload shareddomain.ToolPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return allow, fmt.Errorf("decode tool payload: %w", err)
	}
	call, err := domain.ToolCallOf(payload)
	if err != nil {
		return allow, err
	}
	if call.Kind != shareddomain.ToolKindAgent || call.Model == "" || !domain.IsReviewerAgent(call.AgentType) {
		return allow, nil
	}
	if err := p.deps.SaveReview(ctx, p.deps.Worktree, call.Model); err != nil {
		return shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: reviewRecordFailed + err.Error()}, nil
	}
	return allow, nil
}

type reviewShip struct {
	deps   Deps
	denies metric.Int64Counter
}

func (reviewShip) Name() string { return namePrefix + domain.RuleReviewModels }

func (reviewShip) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventRunTransitionBefore}
}

func (p reviewShip) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	var payload shareddomain.TransitionPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return shareddomain.Verdict{}, fmt.Errorf("decode transition payload: %w", err)
	}
	if payload.Event != shareddomain.TransitionApproveReview {
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
	}
	models, err := p.deps.ReviewModels(ctx, p.deps.Worktree)
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	listed := make(map[shareddomain.Host][]string, len(p.deps.Models))
	for host, roles := range p.deps.Models {
		if list := slices.Clone(roles[string(shareddomain.RoleReviewer)]); len(list) > 0 {
			slices.Sort(list)
			listed[host] = slices.Compact(list)
		}
	}
	reason, denied := domain.DenyReviewModels(models, listed)
	if !denied {
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
	}
	countDeny(ctx, p.denies, domain.RuleReviewModels)
	return deny(reason), nil
}
