package hookcodec

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/jorge07/harness/internal/host/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	eventSessionStart  = "SessionStart"
	eventUserPrompt    = "UserPromptSubmit"
	eventPreToolUse    = "PreToolUse"
	eventPostToolUse   = "PostToolUse"
	eventSubagentStart = "SubagentStart"
	eventStop          = "Stop"
	eventSubagentStop  = "SubagentStop"

	decisionBlock      = "block"
	permissionDecision = "deny"
	emptyOutput        = `{}`
	contextSeparator   = "\n\n"
)

var eventNames = map[domain.HookName]string{
	domain.HookSessionStart:  eventSessionStart,
	domain.HookUserPrompt:    eventUserPrompt,
	domain.HookPreToolUse:    eventPreToolUse,
	domain.HookPostToolUse:   eventPostToolUse,
	domain.HookSubagentStart: eventSubagentStart,
	domain.HookStop:          eventStop,
	domain.HookSubagentStop:  eventSubagentStop,
}

var ErrNoToolName = errors.New("no tool name")

type Codec struct {
	host           shareddomain.Host
	printsStopJSON bool
}

var _ domain.Codec = Codec{}

func NewClaude() Codec { return Codec{host: shareddomain.HostClaude} }

// Codex rejects a Stop or SubagentStop hook that prints no JSON.
func NewCodex() Codec { return Codec{host: shareddomain.HostCodex, printsStopJSON: true} }

type hookInput struct {
	Cwd            string          `json:"cwd"`
	Source         string          `json:"source"`
	Prompt         string          `json:"prompt"`
	ToolName       string          `json:"tool_name"`
	ToolInput      json.RawMessage `json:"tool_input"`
	ToolResponse   json.RawMessage `json:"tool_response"`
	StopHookActive bool            `json:"stop_hook_active"`
	BackgroundTask []struct {
		Type string `json:"type"`
	} `json:"background_tasks"`
	AgentID   string `json:"agent_id"`
	AgentType string `json:"agent_type"`
}

func (c Codec) Host() shareddomain.Host { return c.host }

func (Codec) Capabilities() []shareddomain.EventName { return slices.Clone(domain.AllEvents) }

func (c Codec) Decode(hook domain.HookName, raw []byte) (shareddomain.Envelope, error) {
	event, ok := domain.EventOf(hook)
	if !ok {
		return shareddomain.Envelope{}, fmt.Errorf("%w: %q", domain.ErrUnknownHook, hook)
	}
	var in hookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		return shareddomain.Envelope{}, fmt.Errorf("decode %s input: %w", hook, err)
	}
	if (hook == domain.HookPreToolUse || hook == domain.HookPostToolUse) && in.ToolName == "" {
		return shareddomain.Envelope{}, fmt.Errorf("%w: %s", ErrNoToolName, hook)
	}
	payload, err := json.Marshal(payloadOf(hook, in))
	if err != nil {
		return shareddomain.Envelope{}, fmt.Errorf("encode %s payload: %w", hook, err)
	}
	return shareddomain.Envelope{
		Version: shareddomain.EnvelopeVersion,
		Event:   event,
		Host:    c.host,
		Cwd:     in.Cwd,
		Payload: payload,
	}, nil
}

func payloadOf(hook domain.HookName, in hookInput) any {
	switch hook {
	case domain.HookSessionStart:
		return shareddomain.SessionPayload{Source: in.Source}
	case domain.HookUserPrompt:
		return shareddomain.PromptPayload{Prompt: in.Prompt}
	case domain.HookPreToolUse:
		return toolPayload(in)
	case domain.HookPostToolUse:
		payload := toolPayload(in)
		payload.Response = in.ToolResponse
		return payload
	case domain.HookStop:
		agents := 0
		for _, task := range in.BackgroundTask {
			if shareddomain.CountsAsAgent(task.Type) {
				agents++
			}
		}
		return shareddomain.StopPayload{StopHookActive: in.StopHookActive, BackgroundAgents: agents}
	default:
		return shareddomain.SubagentPayload{AgentID: in.AgentID, AgentType: in.AgentType}
	}
}

func toolPayload(in hookInput) shareddomain.ToolPayload {
	return shareddomain.ToolPayload{
		Tool:  in.ToolName,
		Kind:  domain.ToolKindOf(in.ToolName),
		Input: in.ToolInput,
	}
}

type specificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
}

type hookOutput struct {
	Decision       string          `json:"decision,omitempty"`
	Reason         string          `json:"reason,omitempty"`
	SpecificOutput *specificOutput `json:"hookSpecificOutput,omitempty"`
}

func (c Codec) Encode(hook domain.HookName, verdict shareddomain.Verdict) ([]byte, error) {
	var out hookOutput
	switch {
	case verdict.Kind == shareddomain.VerdictDeny && hook == domain.HookPreToolUse:
		out.SpecificOutput = &specificOutput{
			HookEventName:            eventPreToolUse,
			PermissionDecision:       permissionDecision,
			PermissionDecisionReason: reasonOf(verdict),
		}
	case verdict.Kind == shareddomain.VerdictDeny && (hook == domain.HookUserPrompt || hook == domain.HookStop):
		out.Decision = decisionBlock
		out.Reason = reasonOf(verdict)
	case verdict.Context != "" && !isStop(hook):
		out.SpecificOutput = &specificOutput{HookEventName: eventNames[hook], AdditionalContext: verdict.Context}
	default:
		return c.allowOutput(hook), nil
	}
	return json.Marshal(out)
}

func reasonOf(verdict shareddomain.Verdict) string {
	if verdict.Context == "" {
		return verdict.Reason
	}
	return verdict.Reason + contextSeparator + verdict.Context
}

// A context on a stop makes the host continue the turn, so a stop prints the allow output.
func isStop(hook domain.HookName) bool {
	return hook == domain.HookStop || hook == domain.HookSubagentStop
}

func (c Codec) allowOutput(hook domain.HookName) []byte {
	if c.printsStopJSON && isStop(hook) {
		return []byte(emptyOutput)
	}
	return nil
}
