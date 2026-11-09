package hookcodec_test

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	"github.com/90p-labs/harness/internal/host/infrastructure/hookcodec"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	codexSessionStartInput = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"SessionStart","model":"gpt-6-astra","permission_mode":"default","source":"resume"}`
	codexUserPromptInput   = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"UserPromptSubmit","model":"gpt-6-astra","turn_id":"t1","prompt":"fix the build"}`
	codexPreShellInput     = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"PreToolUse","model":"gpt-6-astra","turn_id":"t1","tool_name":"Bash","tool_use_id":"u1","tool_input":{"command":"go test ./..."}}`
	codexPrePatchInput     = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"PreToolUse","model":"gpt-6-astra","turn_id":"t1","tool_name":"apply_patch","tool_use_id":"u2","tool_input":{"command":"*** Begin Patch\n*** End Patch"}}`
	codexPostToolInput     = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"PostToolUse","model":"gpt-6-astra","turn_id":"t1","tool_name":"Bash","tool_use_id":"u1","tool_input":{"command":"ls"},"tool_response":{"output":"a.go"}}`
	codexStopInput         = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"Stop","model":"gpt-6-astra","turn_id":"t1","stop_hook_active":true,"last_assistant_message":"done"}`
	codexStopTasksInput    = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"Stop","model":"gpt-6-astra","turn_id":"t1","stop_hook_active":false,"background_tasks":[{"id":"task-001","type":"shell","status":"running"},{"id":"task-002","type":"subagent","status":"running"}]}`
	codexSubagentStartIn   = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"SubagentStart","model":"gpt-6-astra","agent_id":"a1","agent_type":"explorer"}`
	codexSubagentStopIn    = `{"session_id":"s1","transcript_path":"/tmp/t.jsonl","cwd":"/work/app","hook_event_name":"SubagentStop","model":"gpt-6-astra","agent_id":"a1","agent_type":"explorer","agent_transcript_path":"/tmp/a.jsonl","stop_hook_active":false,"last_assistant_message":"ok"}`
)

func TestCodexDecode(t *testing.T) {
	codec := hookcodec.NewCodex()
	cases := []struct {
		name    string
		hook    hostdomain.HookName
		input   string
		event   shareddomain.EventName
		payload string
	}{
		{"session start", hostdomain.HookSessionStart, codexSessionStartInput, shareddomain.EventSessionStarted, `{"source":"resume","missing_events":[]}`},
		{"user prompt", hostdomain.HookUserPrompt, codexUserPromptInput, shareddomain.EventPromptSubmitted, `{"prompt":"fix the build"}`},
		{"pre tool shell", hostdomain.HookPreToolUse, codexPreShellInput, shareddomain.EventToolBefore, `{"tool":"Bash","kind":"shell","input":{"command":"go test ./..."}}`},
		{"pre tool edit", hostdomain.HookPreToolUse, codexPrePatchInput, shareddomain.EventToolBefore, `{"tool":"apply_patch","kind":"edit","input":{"command":"*** Begin Patch\n*** End Patch"}}`},
		{"post tool", hostdomain.HookPostToolUse, codexPostToolInput, shareddomain.EventToolAfter, `{"tool":"Bash","kind":"shell","input":{"command":"ls"},"response":{"output":"a.go"}}`},
		{"stop", hostdomain.HookStop, codexStopInput, shareddomain.EventTurnStopping, `{"stop_hook_active":true,"background_agents":0}`},
		{"stop with tasks", hostdomain.HookStop, codexStopTasksInput, shareddomain.EventTurnStopping, `{"stop_hook_active":false,"background_agents":1}`},
		{"subagent start", hostdomain.HookSubagentStart, codexSubagentStartIn, shareddomain.EventSubagentStarted, `{"agent_id":"a1","agent_type":"explorer"}`},
		{"subagent stop", hostdomain.HookSubagentStop, codexSubagentStopIn, shareddomain.EventSubagentStopped, `{"agent_id":"a1","agent_type":"explorer"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := codec.Decode(c.hook, []byte(c.input))
			if err != nil {
				t.Fatalf("Decode error = %v", err)
			}
			if got.Version != shareddomain.EnvelopeVersion || got.Event != c.event ||
				got.Host != shareddomain.HostCodex || got.Cwd != "/work/app" || got.Run != (shareddomain.RunRef{}) {
				t.Fatalf("envelope = %+v", got)
			}
			if string(got.Payload) != c.payload {
				t.Fatalf("payload = %s, want %s", got.Payload, c.payload)
			}
		})
	}

	t.Run("bad input", func(t *testing.T) {
		for _, raw := range []string{"", "{", "[]"} {
			if _, err := codec.Decode(hostdomain.HookStop, []byte(raw)); err == nil {
				t.Fatalf("Decode(%q) gave no error", raw)
			}
		}
	})

	t.Run("tool hook without a tool name", func(t *testing.T) {
		for _, hook := range []hostdomain.HookName{hostdomain.HookPreToolUse, hostdomain.HookPostToolUse} {
			if _, err := codec.Decode(hook, []byte(`{"cwd":"/work/app"}`)); !errors.Is(err, hookcodec.ErrNoToolName) {
				t.Fatalf("Decode(%s) error = %v, want ErrNoToolName", hook, err)
			}
		}
	})

	t.Run("host and capabilities", func(t *testing.T) {
		if codec.Host() != shareddomain.HostCodex {
			t.Fatalf("Host = %q", codec.Host())
		}
		if !slices.Equal(codec.Capabilities(), hostdomain.AllEvents) {
			t.Fatalf("Capabilities = %v", codec.Capabilities())
		}
	})
}

const (
	codexQuestionInput  = `{"questions":[{"id":"plan_approval","header":"Plan","question":"Is the plan ready?","options":[{"label":"Approve (Recommended)","description":"start the work"},{"label":"Change the plan","description":"edit plan.md"}]}]}`
	codexQuestionPrefix = `{"cwd":"/work/app","hook_event_name":"PostToolUse","model":"gpt-6-astra","turn_id":"t1","tool_name":"request_user_input","tool_use_id":"u3","tool_input":` + codexQuestionInput + `,"tool_response":`
)

func TestCodexDecodeSessionID(t *testing.T) {
	codec := hookcodec.NewCodex()
	with, err := codec.Decode(hostdomain.HookSessionStart, []byte(codexSessionStartInput))
	if err != nil || with.SessionID != "s1" {
		t.Fatalf("SessionID = %q, err = %v, want s1", with.SessionID, err)
	}
	without, err := codec.Decode(hostdomain.HookSessionStart, []byte(`{"cwd":"/work/app"}`))
	if err != nil || without.SessionID != "" {
		t.Fatalf("SessionID = %q, err = %v, want empty", without.SessionID, err)
	}
}

func TestQuestionDecodeCodex(t *testing.T) {
	approve := []shareddomain.Answer{{Header: shareddomain.ApprovalHeader, Label: shareddomain.ApproveLabel}}
	cases := []struct {
		name     string
		response string
		want     []shareddomain.Answer
	}{
		{"response as an object", `{"answers":{"plan_approval":{"answers":["Approve (Recommended)"]}}}`, approve},
		{"response as a JSON string", `"{\"answers\":{\"plan_approval\":{\"answers\":[\"Approve\"]}}}"`, approve},
		{"unparsable string", `"cancelled"`, nil},
		{"answer to an unknown id", `{"answers":{"other":{"answers":["Approve"]}}}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			payload, answers := decodedAnswers(t, hookcodec.NewCodex(), hostdomain.HookPostToolUse, codexQuestionPrefix+c.response+`}`)
			if payload.Kind != shareddomain.ToolKindQuestion || !slices.Equal(answers, c.want) {
				t.Fatalf("kind = %q, answers = %+v, want %q and %+v", payload.Kind, answers, shareddomain.ToolKindQuestion, c.want)
			}
		})
	}
}

func TestCodexEncode(t *testing.T) {
	codec := hookcodec.NewCodex()
	allow := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	deny := shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "no way"}
	context := shareddomain.Verdict{Kind: shareddomain.VerdictAllow, Context: "note"}
	denyAfterContext := shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "no way", Context: "note"}
	cases := []struct {
		name    string
		hook    hostdomain.HookName
		verdict shareddomain.Verdict
		want    string
	}{
		{"allow session start", hostdomain.HookSessionStart, allow, ""},
		{"allow user prompt", hostdomain.HookUserPrompt, allow, ""},
		{"allow pre tool", hostdomain.HookPreToolUse, allow, ""},
		{"allow post tool", hostdomain.HookPostToolUse, allow, ""},
		{"allow subagent start", hostdomain.HookSubagentStart, allow, ""},
		{"allow stop", hostdomain.HookStop, allow, `{}`},
		{"allow subagent stop", hostdomain.HookSubagentStop, allow, `{}`},
		{"deny pre tool", hostdomain.HookPreToolUse, deny, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":"no way"}}`},
		{"deny user prompt", hostdomain.HookUserPrompt, deny, `{"decision":"block","reason":"no way"}`},
		{"deny stop", hostdomain.HookStop, deny, `{"decision":"block","reason":"no way"}`},
		{"deny user prompt after a context", hostdomain.HookUserPrompt, denyAfterContext, `{"decision":"block","reason":"no way\n\nnote"}`},
		{"context session start", hostdomain.HookSessionStart, context, `{"hookSpecificOutput":{"hookEventName":"SessionStart","additionalContext":"note"}}`},
		{"context user prompt", hostdomain.HookUserPrompt, context, `{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"note"}}`},
		{"context pre tool", hostdomain.HookPreToolUse, context, `{"hookSpecificOutput":{"hookEventName":"PreToolUse","additionalContext":"note"}}`},
		{"context post tool", hostdomain.HookPostToolUse, context, `{"hookSpecificOutput":{"hookEventName":"PostToolUse","additionalContext":"note"}}`},
		{"context subagent start", hostdomain.HookSubagentStart, context, `{"hookSpecificOutput":{"hookEventName":"SubagentStart","additionalContext":"note"}}`},
		{"context stop", hostdomain.HookStop, context, `{}`},
		{"context subagent stop", hostdomain.HookSubagentStop, context, `{}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := codec.Encode(c.hook, c.verdict)
			if err != nil {
				t.Fatalf("Encode error = %v", err)
			}
			if string(got) != c.want {
				t.Fatalf("Encode = %q, want %q", got, c.want)
			}
			if len(got) > 0 && !json.Valid(got) {
				t.Fatalf("Encode gave invalid JSON: %s", got)
			}
		})
	}
}
