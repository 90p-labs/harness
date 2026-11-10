package guardplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"go.opentelemetry.io/otel/metric"

	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type findingsShip struct {
	deps   Deps
	denies metric.Int64Counter
}

func (findingsShip) Name() string { return namePrefix + domain.RuleReviewFindings }

func (findingsShip) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventRunTransitionBefore}
}

func (p findingsShip) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	var payload shareddomain.TransitionPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return shareddomain.Verdict{}, fmt.Errorf("decode transition payload: %w", err)
	}
	if payload.Event != shareddomain.TransitionApproveReview {
		return allow, nil
	}
	body, err := p.deps.ReadFile(filepath.Join(p.deps.Worktree, shareddomain.RunFindingsFile(envelope.Run.ID)))
	if errors.Is(err, fs.ErrNotExist) {
		return allow, nil
	}
	var findings []shareddomain.ReviewFinding
	if err == nil {
		findings, err = shareddomain.ParseReviewFindings(string(body))
	}
	if err != nil {
		countDeny(ctx, p.denies, domain.RuleReviewFindings)
		return deny(fmt.Sprintf("the findings file is not valid: %s", err)), nil
	}
	reason, denied := domain.DenyOpenFindings(findings, p.deps.StrictMinor)
	if !denied && domain.HasBlockerOrMajor(findings) {
		reasons, err := p.deps.ReworkReasons(ctx, p.deps.Worktree, envelope.Run.ID)
		if err != nil {
			countDeny(ctx, p.denies, domain.RuleReviewFindings)
			return deny(fmt.Sprintf("the rework reasons are not readable: %s", err)), nil
		}
		reason, denied = domain.DenyUnreworkedFindings(findings, reasons)
	}
	if !denied {
		return allow, nil
	}
	countDeny(ctx, p.denies, domain.RuleReviewFindings)
	return deny(reason), nil
}
