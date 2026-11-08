package lintplugin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type Plugin struct {
	engine domain.Engine
}

var _ shareddomain.Plugin = Plugin{}

func New(engine domain.Engine) Plugin {
	return Plugin{engine: engine}
}

func (p Plugin) Name() string { return string(p.engine.Name()) }

func (Plugin) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventLintCheck, shareddomain.EventLintPrepare}
}

func (p Plugin) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	var payload domain.CheckPayload
	if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
		return shareddomain.Verdict{}, fmt.Errorf("decode lint payload: %w", err)
	}
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	if envelope.Event == shareddomain.EventLintPrepare {
		return allow, p.engine.Prepare(envelope.Cwd, payload.Rules)
	}
	findings, err := p.engine.Run(ctx, envelope.Cwd, payload.Rules)
	allow.Findings = findings
	return allow, err
}
