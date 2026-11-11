package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
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

	HookPostToolUseFailure HookName = "post-tool-use-failure"
	HookSessionEnd         HookName = "session-end"
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

	HookPostToolUseFailure: shareddomain.EventToolFailed,
	HookSessionEnd:         shareddomain.EventSessionEnded,
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
	base := shareddomain.ToolBaseName(tool, func(name string) bool {
		_, ok := toolKinds[name]
		return ok || isQuestionTool(name)
	})
	if kind, ok := toolKinds[base]; ok {
		return kind
	}
	if isQuestionTool(base) {
		return shareddomain.ToolKindQuestion
	}
	return shareddomain.ToolKindOther
}

func isQuestionTool(name string) bool {
	for _, question := range shareddomain.QuestionTools {
		if question.Name == name {
			return true
		}
	}
	return false
}

const codexOnlyTool = "apply_patch"

// MatchedTools lists the Claude tools that a guard rule reads, sorted.
func MatchedTools() []string {
	tools := []string{shareddomain.QuestionTools[shareddomain.HostClaude].Name}
	for tool := range toolKinds {
		if tool != codexOnlyTool {
			tools = append(tools, tool)
		}
	}
	sort.Strings(tools)
	return tools
}

// ClaudeMatcher is the matcher of the Claude PreToolUse and PostToolUse hooks.
// It is a regex, so a host that prefixes the tool name, such as mcp__server__life_Bash, still matches.
func ClaudeMatcher() string {
	names := strings.Join(MatchedTools(), "|")
	return "^(" + names + ")$|^mcp__.+_(" + names + ")$"
}
