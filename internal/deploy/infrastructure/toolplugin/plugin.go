package toolplugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jorge07/harness/internal/deploy/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const gateSeparator = "\n"

type render func(ctx context.Context, dir, chart string) ([]byte, error)

type Plugin struct {
	name   string
	detect func(files []string) []shareddomain.ProposedGate
	render render
}

func Helm() Plugin {
	return Plugin{name: domain.ToolHelm, detect: domain.HelmGates, render: Template}
}

func Promtool() Plugin {
	return Plugin{name: domain.ToolPromtool, detect: domain.PromtoolGates}
}

func (p Plugin) Name() string { return p.name }

func (p Plugin) Subscribes() []shareddomain.EventName {
	if p.render == nil {
		return []shareddomain.EventName{shareddomain.EventGatesDetect}
	}
	return []shareddomain.EventName{shareddomain.EventGatesDetect, shareddomain.EventChartRender}
}

func (p Plugin) Handle(ctx context.Context, envelope shareddomain.Envelope) (shareddomain.Verdict, error) {
	switch envelope.Event {
	case shareddomain.EventGatesDetect:
		return p.proposeGates(envelope.Payload)
	case shareddomain.EventChartRender:
		var payload shareddomain.ChartRenderPayload
		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return shareddomain.Verdict{}, fmt.Errorf("%s payload: %w", envelope.Event, err)
		}
		out, err := p.render(ctx, envelope.Cwd, payload.Chart)
		if err != nil {
			return shareddomain.Verdict{}, err
		}
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: string(out)}, nil
	}
	return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
}

func (p Plugin) proposeGates(raw json.RawMessage) (shareddomain.Verdict, error) {
	var payload shareddomain.GatesDetectPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return shareddomain.Verdict{}, fmt.Errorf("%s payload: %w", shareddomain.EventGatesDetect, err)
	}
	var lines []string
	for _, gate := range p.detect(payload.Files) {
		line, err := json.Marshal(gate)
		if err != nil {
			return shareddomain.Verdict{}, err
		}
		lines = append(lines, string(line))
	}
	return shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: strings.Join(lines, gateSeparator)}, nil
}
