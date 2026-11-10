package domain_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/90p-labs/harness/internal/host/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type fakeCodec struct {
	events []shareddomain.EventName
}

func (f fakeCodec) Host() shareddomain.Host { return shareddomain.HostCodex }

func (f fakeCodec) Capabilities() []shareddomain.EventName { return f.events }

func (f fakeCodec) Decode(domain.HookName, []byte) (shareddomain.Envelope, error) {
	return shareddomain.Envelope{}, nil
}

func (f fakeCodec) Encode(domain.HookName, shareddomain.Verdict) ([]byte, error) {
	return nil, nil
}

func TestCapabilities(t *testing.T) {
	t.Run("event of each hook", func(t *testing.T) {
		cases := []struct {
			hook domain.HookName
			want shareddomain.EventName
		}{
			{domain.HookSessionStart, shareddomain.EventSessionStarted},
			{domain.HookUserPrompt, shareddomain.EventPromptSubmitted},
			{domain.HookPreToolUse, shareddomain.EventToolBefore},
			{domain.HookPostToolUse, shareddomain.EventToolAfter},
			{domain.HookStop, shareddomain.EventTurnStopping},
			{domain.HookSubagentStart, shareddomain.EventSubagentStarted},
			{domain.HookSubagentStop, shareddomain.EventSubagentStopped},
		}
		for _, c := range cases {
			got, ok := domain.EventOf(c.hook)
			if !ok || got != c.want {
				t.Fatalf("EventOf(%q) = %q, %v, want %q", c.hook, got, ok, c.want)
			}
		}
		if _, ok := domain.EventOf("pre-compact"); ok {
			t.Fatal("EventOf accepted an unknown hook")
		}
	})

	t.Run("parse hook name", func(t *testing.T) {
		got, err := domain.ParseHookName("pre-tool-use")
		if err != nil || got != domain.HookPreToolUse {
			t.Fatalf("ParseHookName = %q, %v", got, err)
		}
		if _, err := domain.ParseHookName("PreToolUse"); !errors.Is(err, domain.ErrUnknownHook) {
			t.Fatalf("ParseHookName error = %v, want ErrUnknownHook", err)
		}
	})

	t.Run("all events follow the hook order", func(t *testing.T) {
		want := []shareddomain.EventName{
			shareddomain.EventSessionStarted,
			shareddomain.EventPromptSubmitted,
			shareddomain.EventToolBefore,
			shareddomain.EventToolAfter,
			shareddomain.EventTurnStopping,
			shareddomain.EventSubagentStarted,
			shareddomain.EventSubagentStopped,
		}
		if !slices.Equal(domain.AllEvents, want) {
			t.Fatalf("AllEvents = %v, want %v", domain.AllEvents, want)
		}
	})

	t.Run("missing events", func(t *testing.T) {
		full := fakeCodec{events: slices.Clone(domain.AllEvents)}
		if got := domain.MissingEvents(full); len(got) != 0 {
			t.Fatalf("MissingEvents of a full codec = %v, want none", got)
		}

		partial := fakeCodec{events: []shareddomain.EventName{
			shareddomain.EventSessionStarted,
			shareddomain.EventPromptSubmitted,
			shareddomain.EventToolBefore,
			shareddomain.EventTurnStopping,
		}}
		want := []shareddomain.EventName{
			shareddomain.EventToolAfter,
			shareddomain.EventSubagentStarted,
			shareddomain.EventSubagentStopped,
		}
		if got := domain.MissingEvents(partial); !slices.Equal(got, want) {
			t.Fatalf("MissingEvents = %v, want %v", got, want)
		}
	})

	t.Run("tool kind of each tool name", func(t *testing.T) {
		cases := []struct {
			tool string
			want shareddomain.ToolKind
		}{
			{"Bash", shareddomain.ToolKindShell},
			{"Edit", shareddomain.ToolKindEdit},
			{"MultiEdit", shareddomain.ToolKindEdit},
			{"apply_patch", shareddomain.ToolKindEdit},
			{"Write", shareddomain.ToolKindWrite},
			{"Read", shareddomain.ToolKindRead},
			{"Grep", shareddomain.ToolKindSearch},
			{"Glob", shareddomain.ToolKindSearch},
			{"Agent", shareddomain.ToolKindAgent},
			{"WebFetch", shareddomain.ToolKindOther},
			{"shell", shareddomain.ToolKindOther},
			{"", shareddomain.ToolKindOther},
		}
		for _, c := range cases {
			if got := domain.ToolKindOf(c.tool); got != c.want {
				t.Fatalf("ToolKindOf(%q) = %q, want %q", c.tool, got, c.want)
			}
		}
	})

	t.Run("background agent counting", func(t *testing.T) {
		cases := map[string]bool{
			shareddomain.TaskTypeSubagent:     true,
			shareddomain.TaskTypeWorkflow:     true,
			shareddomain.TaskTypeTeammate:     true,
			shareddomain.TaskTypeCloudSession: true,
			shareddomain.TaskTypeShell:        true,
			"":                                false,
		}
		for taskType, want := range cases {
			if got := shareddomain.CountsAsAgent(taskType); got != want {
				t.Fatalf("CountsAsAgent(%q) = %v, want %v", taskType, got, want)
			}
		}
	})

	t.Run("payload json", func(t *testing.T) {
		session, err := json.Marshal(shareddomain.SessionPayload{Source: "startup"})
		if err != nil {
			t.Fatal(err)
		}
		if want := `{"source":"startup","missing_events":[]}`; string(session) != want {
			t.Fatalf("session payload = %s, want %s", session, want)
		}

		tool, err := json.Marshal(shareddomain.ToolPayload{
			Tool:  "Bash",
			Kind:  shareddomain.ToolKindShell,
			Input: json.RawMessage(`{"command":"ls"}`),
		})
		if err != nil {
			t.Fatal(err)
		}
		if want := `{"tool":"Bash","kind":"shell","input":{"command":"ls"}}`; string(tool) != want {
			t.Fatalf("tool payload = %s, want %s", tool, want)
		}

		stop, err := json.Marshal(shareddomain.StopPayload{BackgroundAgents: 2})
		if err != nil {
			t.Fatal(err)
		}
		if want := `{"stop_hook_active":false,"background_agents":2}`; string(stop) != want {
			t.Fatalf("stop payload = %s, want %s", stop, want)
		}
	})
}
