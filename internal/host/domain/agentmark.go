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
)

type MarkStep struct {
	Kind        MarkKind
	Close, Open bool
}

var markSteps = map[HookName]MarkStep{
	HookUserPrompt:    {Kind: MarkTurn, Close: true, Open: true},
	HookStop:          {Kind: MarkTurn, Close: true},
	HookSubagentStart: {Kind: MarkSubagent, Open: true},
	HookSubagentStop:  {Kind: MarkSubagent, Close: true},
}

func MarkStepOf(hook HookName, verdict shareddomain.VerdictKind) (MarkStep, bool) {
	if verdict == shareddomain.VerdictDeny {
		return MarkStep{}, false
	}
	step, ok := markSteps[hook]
	return step, ok
}

// Open keeps the first time of an open mark. Close returns the start time and
// deletes the mark, false when no mark is open.
type AgentMarks interface {
	Open(ctx context.Context, kind MarkKind, id string, at time.Time) error
	Close(ctx context.Context, kind MarkKind, id string) (time.Time, bool, error)
}
