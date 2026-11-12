package guardplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"

	"go.opentelemetry.io/otel/metric"

	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const nameReviewRecorder = namePrefix + "review-recorder"

type reviewRecorder struct {
	deps Deps
}

func (reviewRecorder) Name() string { return nameReviewRecorder }

func (reviewRecorder) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventSubagentStopped}
}

func (p reviewRecorder) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	var payload shareddomain.SubagentPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return allow, fmt.Errorf("decode subagent payload: %w", err)
	}
	if !domain.IsReviewerAgent(payload.AgentType) {
		return allow, nil
	}
	// From here on, every return without a saved review says why, so a ship veto for a missing review has a cause.
	switch {
	case envelope.Run.ID == "":
		return allow, fmt.Errorf("reviewer %s stopped with no active run, no review recorded", payload.AgentID)
	case envelope.Run.Phase != shareddomain.PhaseReviewing:
		return allow, fmt.Errorf("reviewer %s stopped in phase %s, not %s, no review recorded", payload.AgentID, envelope.Run.Phase, shareddomain.PhaseReviewing)
	case payload.TranscriptPath == "":
		return allow, fmt.Errorf("reviewer %s stopped with no transcript path, no review recorded", payload.AgentID)
	}
	transcript, err := p.deps.ReadFile(payload.TranscriptPath)
	if err != nil {
		return allow, fmt.Errorf("read the reviewer transcript: %w", err)
	}
	base, err := p.deps.DefaultBranch(ctx, p.deps.Worktree)
	if err != nil {
		return allow, fmt.Errorf("find the default branch: %w", err)
	}
	changed, err := p.deps.ChangedFiles(ctx, p.deps.Worktree, base)
	if err != nil {
		return allow, fmt.Errorf("list the files changed since %s: %w", base, err)
	}
	modelID, read := domain.FilesRead(transcript, p.deps.Worktree, changed)
	model, ok := p.recordedModel(envelope.Host, modelID)
	switch {
	case modelID == "":
		return allow, fmt.Errorf("no model in transcript %s, no review recorded", payload.TranscriptPath)
	case !ok:
		return allow, fmt.Errorf("model %q is not listed under roles reviewer for host %s, no review recorded", modelID, envelope.Host)
	}
	patchID, err := p.deps.PatchID(ctx, p.deps.Worktree, base)
	if err != nil {
		return allow, fmt.Errorf("compute the patch id against %s: %w", base, err)
	}
	review := domain.Review{Model: model, PatchID: patchID, RunID: envelope.Run.ID, FilesRead: read}
	if err := p.deps.SaveReview(ctx, p.deps.Worktree, review); err != nil {
		return allow, fmt.Errorf("save the review of %s: %w", model, err)
	}
	return allow, nil
}

func (p reviewRecorder) recordedModel(host shareddomain.Host, modelID string) (string, bool) {
	listed := p.deps.Models[host][string(shareddomain.RoleReviewer)]
	if len(listed) == 0 {
		return modelID, modelID != ""
	}
	return domain.ListedModelFor(listed, modelID)
}

type reviewShip struct {
	deps   Deps
	denies metric.Int64Counter
}

func (reviewShip) Name() string { return namePrefix + domain.RuleReviewModels }

func (reviewShip) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventRunTransitionBefore}
}

// An empty patch id never matches, so the head and run matches still work without a merge base.
func (p reviewShip) patchID(ctx context.Context) string {
	base, err := p.deps.DefaultBranch(ctx, p.deps.Worktree)
	if err != nil {
		return ""
	}
	patchID, err := p.deps.PatchID(ctx, p.deps.Worktree, base)
	if err != nil {
		return ""
	}
	return patchID
}

func (p reviewShip) hasFindings(runID string) bool {
	_, err := p.deps.ReadFile(filepath.Join(p.deps.Worktree, shareddomain.RunFindingsFile(runID)))
	return err == nil
}

func (p reviewShip) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	var payload shareddomain.TransitionPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return shareddomain.Verdict{}, fmt.Errorf("decode transition payload: %w", err)
	}
	if payload.Event != shareddomain.TransitionApproveReview {
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
	}
	scope := domain.ReviewScope{PatchID: p.patchID(ctx)}
	if !p.deps.StrictMinor && p.hasFindings(envelope.Run.ID) {
		scope.RunID = envelope.Run.ID
	}
	reviews, err := p.deps.Reviews(ctx, p.deps.Worktree, scope)
	if err != nil {
		return shareddomain.Verdict{}, err
	}
	counted, unread := domain.ReviewedModels(reviews)
	listed := make(map[shareddomain.Host][]string, len(p.deps.Models))
	for host, roles := range p.deps.Models {
		if list := slices.Clone(roles[string(shareddomain.RoleReviewer)]); len(list) > 0 {
			slices.Sort(list)
			listed[host] = slices.Compact(list)
		}
	}
	result := FinalPanel(ctx, p.deps, envelope.Run.ID, counted)
	if len(result.List) > 0 && len(result.Panel) < len(result.List) {
		panel := slices.Clone(result.Panel)
		slices.Sort(panel)
		listed[result.Host] = panel
	}
	// Check only the host FinalPanel chose, so a shrunk panel cannot hand the check to another host.
	if len(result.List) > 0 {
		listed = map[shareddomain.Host][]string{result.Host: listed[result.Host]}
	}
	reason, denied := domain.DenyReviewModels(counted, unread, listed, domain.ReviewFloor(result.Light))
	if !denied {
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
	}
	countDeny(ctx, p.denies, domain.RuleReviewModels)
	return deny(reason + " (" + result.Reason + ")"), nil
}
