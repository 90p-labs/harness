package hookcodec_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/jorge07/harness/internal/host/domain"
	"github.com/jorge07/harness/internal/host/infrastructure/hookcodec"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	sessionStartInput = `{"session_id":"abc123","transcript_path":"/t.jsonl","cwd":"/work","hook_event_name":"SessionStart","source":"resume","model":"claude-opus-4-1"}`
	userPromptInput   = `{"session_id":"abc123","transcript_path":"/t.jsonl","cwd":"/work","permission_mode":"default","hook_event_name":"UserPromptSubmit","prompt":"fix the flaky test"}`
	preToolUseInput   = `{"session_id":"abc123","transcript_path":"/t.jsonl","cwd":"/work","permission_mode":"default","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"go test ./...","description":"run tests"},"tool_use_id":"toolu_01"}`
	postToolUseInput  = `{"session_id":"abc123","transcript_path":"/t.jsonl","cwd":"/work","permission_mode":"default","hook_event_name":"PostToolUse","tool_name":"Write","tool_input":{"file_path":"/work/a.go","content":"package a"},"tool_response":{"filePath":"/work/a.go","success":true},"tool_use_id":"toolu_02"}`
	stopInput         = `{"session_id":"abc123","transcript_path":"/t.jsonl","cwd":"/work","permission_mode":"default","hook_event_name":"Stop","stop_hook_active":true,"last_assistant_message":"done","background_tasks":[{"id":"task-001","type":"shell","status":"running"},{"id":"task-002","type":"subagent","status":"running","agent_type":"Explore"}],"session_crons":[]}`
	stopNoTasksInput  = `{"session_id":"abc123","transcript_path":"/t.jsonl","cwd":"/work","hook_event_name":"Stop","stop_hook_active":false}`
	subagentStart     = `{"session_id":"abc123","transcript_path":"/t.jsonl","cwd":"/work","hook_event_name":"SubagentStart","agent_id":"agent-7","agent_type":"Explore"}`
	subagentStop      = `{"session_id":"abc123","transcript_path":"/t.jsonl","cwd":"/work","permission_mode":"default","hook_event_name":"SubagentStop","stop_hook_active":false,"agent_id":"agent-7","agent_type":"Explore","agent_transcript_path":"/a.jsonl","last_assistant_message":"found it"}`
)

func TestClaudeDecode(t *testing.T) {
	cases := []struct {
		name    string
		hook    domain.HookName
		input   string
		event   shareddomain.EventName
		payload string
	}{
		{"session start", domain.HookSessionStart, sessionStartInput, shareddomain.EventSessionStarted, `{"source":"resume","missing_events":[]}`},
		{"user prompt", domain.HookUserPrompt, userPromptInput, shareddomain.EventPromptSubmitted, `{"prompt":"fix the flaky test"}`},
		{"pre tool use", domain.HookPreToolUse, preToolUseInput, shareddomain.EventToolBefore, `{"tool":"Bash","kind":"shell","input":{"command":"go test ./...","description":"run tests"}}`},
		{"post tool use", domain.HookPostToolUse, postToolUseInput, shareddomain.EventToolAfter, `{"tool":"Write","kind":"write","input":{"file_path":"/work/a.go","content":"package a"},"response":{"filePath":"/work/a.go","success":true}}`},
		{"stop with tasks", domain.HookStop, stopInput, shareddomain.EventTurnStopping, `{"stop_hook_active":true,"background_agents":1}`},
		{"stop without tasks", domain.HookStop, stopNoTasksInput, shareddomain.EventTurnStopping, `{"stop_hook_active":false,"background_agents":0}`},
		{"subagent start", domain.HookSubagentStart, subagentStart, shareddomain.EventSubagentStarted, `{"agent_id":"agent-7","agent_type":"Explore"}`},
		{"subagent stop", domain.HookSubagentStop, subagentStop, shareddomain.EventSubagentStopped, `{"agent_id":"agent-7","agent_type":"Explore"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := hookcodec.NewClaude().Decode(c.hook, []byte(c.input))
			if err != nil {
				t.Fatal(err)
			}
			if got.Version != shareddomain.EnvelopeVersion || got.Event != c.event || got.Host != shareddomain.HostClaude || got.Cwd != "/work" || got.Run != (shareddomain.RunRef{}) {
				t.Fatalf("envelope = %+v", got)
			}
			if string(got.Payload) != c.payload {
				t.Fatalf("payload = %s, want %s", got.Payload, c.payload)
			}
		})
	}

	t.Run("bad input", func(t *testing.T) {
		for _, input := range []string{"", "not json", "{"} {
			if _, err := hookcodec.NewClaude().Decode(domain.HookStop, []byte(input)); err == nil {
				t.Fatalf("Decode(%q) returned no error", input)
			}
		}
	})

	t.Run("tool hook without a tool name", func(t *testing.T) {
		for _, hook := range []domain.HookName{domain.HookPreToolUse, domain.HookPostToolUse} {
			if _, err := hookcodec.NewClaude().Decode(hook, []byte(`{"cwd":"/work"}`)); !errors.Is(err, hookcodec.ErrNoToolName) {
				t.Fatalf("Decode(%s) error = %v, want ErrNoToolName", hook, err)
			}
		}
	})
}

func TestClaudeEncode(t *testing.T) {
	deny := shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "no"}
	context := shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: "note"}
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	denyAfterContext := shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "no", Context: "note"}

	cases := []struct {
		name    string
		hook    domain.HookName
		verdict shareddomain.Verdict
		want    string
	}{
		{"pre tool use allow", domain.HookPreToolUse, allow, ""},
		{"pre tool use deny", domain.HookPreToolUse, deny, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"no"}}`},
		{"pre tool use context", domain.HookPreToolUse, context, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"note"}}`},
		{"pre tool use deny after a context", domain.HookPreToolUse, denyAfterContext, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"no\n\nnote"}}`},
		{"user prompt allow", domain.HookUserPrompt, allow, ""},
		{"user prompt deny", domain.HookUserPrompt, deny, `{"decision":"block","reason":"no"}`},
		{"user prompt context", domain.HookUserPrompt, context, `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"note"}}`},
		{"stop allow", domain.HookStop, allow, ""},
		{"stop deny", domain.HookStop, deny, `{"decision":"block","reason":"no"}`},
		{"stop context", domain.HookStop, context, ""},
		{"session start allow", domain.HookSessionStart, allow, ""},
		{"session start context", domain.HookSessionStart, context, `{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"note"}}`},
		{"post tool use allow", domain.HookPostToolUse, allow, ""},
		{"post tool use context", domain.HookPostToolUse, context, `{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"note"}}`},
		{"subagent start allow", domain.HookSubagentStart, allow, ""},
		{"subagent start context", domain.HookSubagentStart, context, `{"hookSpecificOutput":{"hookEventName":"SubagentStart","additionalContext":"note"}}`},
		{"subagent stop allow", domain.HookSubagentStop, allow, ""},
		{"subagent stop context", domain.HookSubagentStop, context, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := hookcodec.NewClaude().Encode(c.hook, c.verdict)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != c.want {
				t.Fatalf("output = %s, want %s", got, c.want)
			}
			if c.want != "" && !json.Valid(got) {
				t.Fatalf("output is not valid JSON: %s", got)
			}
		})
	}
}

func TestClaudeIdentity(t *testing.T) {
	codec := hookcodec.NewClaude()
	if codec.Host() != shareddomain.HostClaude {
		t.Fatalf("Host = %q", codec.Host())
	}
	if !slices.Equal(codec.Capabilities(), domain.AllEvents) {
		t.Fatalf("Capabilities = %v", codec.Capabilities())
	}
	if domain.MissingEvents(codec) == nil || len(domain.MissingEvents(codec)) != 0 {
		t.Fatalf("MissingEvents = %v", domain.MissingEvents(codec))
	}
}
