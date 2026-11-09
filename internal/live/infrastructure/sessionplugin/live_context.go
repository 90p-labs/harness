package sessionplugin

import (
	"context"
	"strings"
	"time"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	Name         = "live.session"
	CheckTimeout = 20 * time.Second

	linePrefix      = "live: "
	checkFailedLine = linePrefix + "check failed: "
	lineSeparator   = "\n"
)

type Lines func(ctx context.Context) ([]string, error)

type Plugin struct {
	lines Lines
}

var _ shareddomain.Plugin = Plugin{}

func New(lines Lines) Plugin {
	return Plugin{lines: lines}
}

func (Plugin) Name() string { return Name }

func (Plugin) Subscribes() []shareddomain.EventName {
	return []shareddomain.EventName{shareddomain.EventSessionStarted}
}

func (p Plugin) Handle(ctx context.Context, _ shareddomain.Envelope) (shareddomain.Verdict, error) {
	ctx, cancel := context.WithTimeout(ctx, CheckTimeout)
	defer cancel()
	lines, err := p.lines(ctx)
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		out = append(out, linePrefix+line)
	}
	if err != nil && len(lines) == 0 {
		out = append(out, checkFailedLine+strings.ReplaceAll(err.Error(), lineSeparator, lineSeparator+linePrefix))
	}
	if len(out) == 0 {
		return shareddomain.Verdict{Kind: shareddomain.VerdictAllow}, nil
	}
	return shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: strings.Join(out, lineSeparator)}, nil
}
