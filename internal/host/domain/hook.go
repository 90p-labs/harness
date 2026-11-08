package domain

import (
	"errors"
	"fmt"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type HookName string

const (
	HookSessionStart  HookName = "session-start"
	HookUserPrompt    HookName = "user-prompt"
	HookPreToolUse    HookName = "pre-tool-use"
	HookPostToolUse   HookName = "post-tool-use"
	HookStop          HookName = "stop"
	HookSubagentStart HookName = "subagent-start"
	HookSubagentStop  HookName = "subagent-stop"
)

var ErrUnknownHook = errors.New("unknown hook")

var hookEvents = map[HookName]shareddomain.EventName{
	HookSessionStart:  shareddomain.EventSessionStarted,
	HookUserPrompt:    shareddomain.EventPromptSubmitted,
	HookPreToolUse:    shareddomain.EventToolBefore,
	HookPostToolUse:   shareddomain.EventToolAfter,
	HookStop:          shareddomain.EventTurnStopping,
	HookSubagentStart: shareddomain.EventSubagentStarted,
	HookSubagentStop:  shareddomain.EventSubagentStopped,
}

func EventOf(hook HookName) (shareddomain.EventName, bool) {
	event, ok := hookEvents[hook]
	return event, ok
}

func ParseHookName(s string) (HookName, error) {
	hook := HookName(s)
	if _, ok := hookEvents[hook]; !ok {
		return "", fmt.Errorf("%w: %q", ErrUnknownHook, s)
	}
	return hook, nil
}

var toolKinds = map[string]shareddomain.ToolKind{
	"Bash":        shareddomain.ToolKindShell,
	"Edit":        shareddomain.ToolKindEdit,
	"MultiEdit":   shareddomain.ToolKindEdit,
	"apply_patch": shareddomain.ToolKindEdit,
	"Write":       shareddomain.ToolKindWrite,
	"Read":        shareddomain.ToolKindRead,
	"Grep":        shareddomain.ToolKindSearch,
	"Glob":        shareddomain.ToolKindSearch,
	"Agent":       shareddomain.ToolKindAgent,
}

func ToolKindOf(tool string) shareddomain.ToolKind {
	if kind, ok := toolKinds[tool]; ok {
		return kind
	}
	return shareddomain.ToolKindOther
}
