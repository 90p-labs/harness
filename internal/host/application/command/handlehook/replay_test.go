package handlehook_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/host/application/command/handlehook"
	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const replaySession = "e1691a5d"

type replayEvent struct {
	ms    int
	hook  hostdomain.HookName
	agent string
	kind  string
}

type replayWindow struct{ from, to int }

func mainEvents(hook hostdomain.HookName, ms ...int) []replayEvent {
	events := make([]replayEvent, len(ms))
	for i, at := range ms {
		events[i] = replayEvent{ms: at, hook: hook}
	}
	return events
}

func replayTrace() []replayEvent {
	prompt := func(ms ...int) []replayEvent { return mainEvents(hostdomain.HookUserPrompt, ms...) }
	stop := func(ms ...int) []replayEvent { return mainEvents(hostdomain.HookStop, ms...) }
	events := slices.Concat(
		stop(36525), prompt(112690), stop(465573), prompt(832389, 924509, 939398), stop(973394), prompt(984485), stop(989696),
		prompt(1033027), stop(1081816), prompt(1092464), stop(1095758), prompt(1108457), stop(1110357), prompt(1196539), stop(1238794),
		prompt(1241584), stop(1243808), prompt(1243923, 1243983, 1244052, 1255853, 1255927), stop(1260681), prompt(1270789, 1275992),
		stop(1303310), prompt(1310349), stop(1312621), prompt(1313490), stop(1315564), prompt(1328870),
		stop(1502685), prompt(1542525), stop(1544670), prompt(1697612), stop(1701100), prompt(1966386),
		[]replayEvent{
			{464008, hostdomain.HookSubagentStart, "a8237b960ce943e67", "advisor"},
			{832456, hostdomain.HookSubagentStop, "a8237b960ce943e67", "advisor"},
			{1077845, hostdomain.HookSubagentStart, "a70817b38614bb8ad", "reviewer"},
			{1196613, hostdomain.HookSubagentStop, "a70817b38614bb8ad", "reviewer"},
			{1499360, hostdomain.HookSubagentStart, "a1deb1e879fae01f5", "reviewer"},
			{1966515, hostdomain.HookSubagentStop, "a1deb1e879fae01f5", "reviewer"},
		},
	)
	slices.SortStableFunc(events, func(a, b replayEvent) int { return a.ms - b.ms })
	return events
}

func replayInput(e replayEvent) string {
	if e.agent == "" {
		return `{"session_id":"` + replaySession + `","cwd":"/work","stop_hook_active":false,"prompt":"go"}`
	}
	return fmt.Sprintf(`{"session_id":%q,"cwd":"/work","agent_id":%q,"agent_type":%q}`, replaySession, e.agent, e.kind)
}

func TestReplayTraceF999(t *testing.T) {
	gaps := []replayWindow{
		{620074, 809583}, {1714657, 1881095}, {241290, 343293}, {1110371, 1175640}, {1594647, 1655075},
	}
	f := newAgentFixture(allowAll())
	root := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{9}, SpanID: trace.SpanID{8}, TraceFlags: trace.FlagsSampled, Remote: true,
	})
	ctx := trace.ContextWithRemoteSpanContext(context.Background(), root)

	for _, e := range replayTrace() {
		f.clock = clockBase.Add(time.Duration(e.ms) * time.Millisecond)
		f.handler.Handle(ctx, handlehook.Command{
			Host: shareddomain.HostClaude, Hook: e.hook, Input: strings.NewReader(replayInput(e)),
		})
	}

	spans := f.agentSpans()
	for _, span := range spans {
		if span.Parent().SpanID() != root.SpanID() {
			t.Errorf("span %s has parent %s, want the root span", span.Name(), span.Parent().SpanID())
		}
	}
	for _, gap := range gaps {
		from := clockBase.Add(time.Duration(gap.from) * time.Millisecond)
		to := clockBase.Add(time.Duration(gap.to) * time.Millisecond)
		covered := false
		for _, span := range spans {
			covered = covered || !span.StartTime().After(from) && !span.EndTime().Before(to)
		}
		if !covered {
			t.Errorf("gap %d to %d ms lies in no agent span", gap.from, gap.to)
		}
	}
}
