package domain

import (
	"context"
	"time"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type MarkKind string

const (
	MarkTurn     MarkKind = "turn"
	MarkSubagent MarkKind = "subagent"
	MarkTool     MarkKind = "tool"
)

// Session closes every open mark of the session, not one by id.
type MarkStep struct {
	Kind                 MarkKind
	Close, Open, Session bool
}

var markSteps = map[HookName]MarkStep{
	HookUserPrompt:    {Kind: MarkTurn, Close: true, Open: true},
	HookStop:          {Kind: MarkTurn, Close: true},
	HookSubagentStart: {Kind: MarkSubagent, Open: true},
	HookSubagentStop:  {Kind: MarkSubagent, Close: true},
}

var claudeToolSteps = map[HookName]MarkStep{
	HookPreToolUse:         {Kind: MarkTool, Open: true},
	HookPostToolUse:        {Kind: MarkTool, Close: true},
	HookPostToolUseFailure: {Kind: MarkTool, Close: true},
	HookSessionEnd:         {Kind: MarkTool, Close: true, Session: true},
}

func MarkStepOf(host shareddomain.Host, hook HookName, verdict shareddomain.VerdictKind) (MarkStep, bool) {
	if verdict == shareddomain.VerdictDeny {
		return MarkStep{}, false
	}
	if step, ok := markSteps[hook]; ok {
		return step, true
	}
	if host != shareddomain.HostClaude {
		return MarkStep{}, false
	}
	step, ok := claudeToolSteps[hook]
	return step, ok
}

type ToolMark struct {
	ID, SessionID, Tool string
	StartedAt           time.Time
}

// Open keeps the first time of an open mark. Close returns the start time and
// deletes the mark, false when no mark is open.
type AgentMarks interface {
	Open(ctx context.Context, kind MarkKind, id string, at time.Time) error
	Close(ctx context.Context, kind MarkKind, id string) (time.Time, bool, error)
	OpenTool(ctx context.Context, mark ToolMark) error
	CloseSession(ctx context.Context, sessionID string) ([]ToolMark, error)
}
