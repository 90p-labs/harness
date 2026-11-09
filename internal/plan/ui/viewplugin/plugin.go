package viewplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/90p-labs/harness/internal/plan/application/command/renderview"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const Name = "plan.view"

type Render func(ctx context.Context, c renderview.Command) (string, error)

type LiveRound func(ctx context.Context) (int, error)

type Plugin struct {
	worktree  string
	render    Render
	liveRound LiveRound
}

var _ shareddomain.Plugin = Plugin{}

func New(worktree string, render Render, liveRound LiveRound) Plugin {
	return Plugin{worktree: worktree, render: render, liveRound: liveRound}
}

func (Plugin) Name() string { return Name }

func (Plugin) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventRunTransitionAfter}
}

func (p Plugin) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	var payload shareddomain.TransitionPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return allow, fmt.Errorf("decode transition payload: %w", err)
	}
	if payload.To != shareddomain.PhaseAwaitingApproval {
		return allow, nil
	}
	round, roundErr := p.liveRound(ctx)
	if roundErr != nil {
		round = 0
	}
	_, err := p.render(ctx, renderview.Command{
		Path:      filepath.Join(p.worktree, filepath.FromSlash(shareddomain.RunPlanFile(envelope.Run.ID))),
		RunID:     envelope.Run.ID,
		Phase:     payload.To,
		LiveRound: round,
		Proofs:    true,
	})
	return allow, errors.Join(roundErr, err)
}
