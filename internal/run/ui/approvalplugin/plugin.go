package approvalplugin

import (
	"context"
	"encoding/json"
	"fmt"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	Name            = "run.approval"
	RecordedContext = "plan approval recorded"
)

type Record func(ctx context.Context, runID string) error

type Plugin struct {
	record Record
}

var _ shareddomain.Plugin = Plugin{}

func New(record Record) Plugin {
	return Plugin{record: record}
}

func (Plugin) Name() string { return Name }

func (Plugin) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventToolAfter}
}

func (p Plugin) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	if envelope.Run.Phase != shareddomain.PhaseAwaitingApproval {
		return allow, nil
	}
	var payload shareddomain.ToolPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return allow, fmt.Errorf("decode tool payload: %w", err)
	}
	if payload.Kind != shareddomain.ToolKindQuestion || !shareddomain.ApprovesPlan(payload.Answers) {
		return allow, nil
	}
	if err := p.record(ctx, envelope.Run.ID); err != nil {
		return allow, err
	}
	return shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: RecordedContext}, nil
}
