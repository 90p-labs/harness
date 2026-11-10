package marksplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const Name = "host.marks"

var closedPhases = []string{shareddomain.PhaseDone, shareddomain.PhaseAbandoned}

type DeleteRun func(ctx context.Context, runID string) error

type Plugin struct {
	deleteRun DeleteRun
}

var _ shareddomain.Plugin = Plugin{}

func New(deleteRun DeleteRun) Plugin {
	return Plugin{deleteRun: deleteRun}
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
	if !slices.Contains(closedPhases, payload.To) {
		return allow, nil
	}
	return allow, p.deleteRun(ctx, envelope.Run.ID)
}
