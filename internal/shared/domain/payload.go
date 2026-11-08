package domain

import (
	"encoding/json"
	"slices"
)

type ToolKind string

const (
	ToolKindShell  ToolKind = "shell"
	ToolKindEdit   ToolKind = "edit"
	ToolKindWrite  ToolKind = "write"
	ToolKindRead   ToolKind = "read"
	ToolKindSearch ToolKind = "search"
	ToolKindAgent  ToolKind = "agent"
	ToolKindOther  ToolKind = "other"
)

const (
	TaskTypeSubagent     = "subagent"
	TaskTypeWorkflow     = "workflow"
	TaskTypeTeammate     = "teammate"
	TaskTypeCloudSession = "cloud session"
)

var backgroundAgentTypes = []string{
	TaskTypeSubagent,
	TaskTypeWorkflow,
	TaskTypeTeammate,
	TaskTypeCloudSession,
}

func CountsAsAgent(taskType string) bool {
	return slices.Contains(backgroundAgentTypes, taskType)
}

type SessionPayload struct {
	Source        string      `json:"source"`
	MissingEvents []EventName `json:"missing_events"`
}

func (p SessionPayload) MarshalJSON() ([]byte, error) {
	type plain SessionPayload
	if p.MissingEvents == nil {
		p.MissingEvents = []EventName{}
	}
	return json.Marshal(plain(p))
}

type PromptPayload struct {
	Prompt string `json:"prompt"`
}

type ToolPayload struct {
	Tool     string          `json:"tool"`
	Kind     ToolKind        `json:"kind"`
	Input    json.RawMessage `json:"input"`
	Response json.RawMessage `json:"response,omitempty"`
}

type StopPayload struct {
	StopHookActive   bool `json:"stop_hook_active"`
	BackgroundAgents int  `json:"background_agents"`
}

type GatesDetectPayload struct {
	Files []string `json:"files"`
}

type ProposedGate struct {
	ID      string   `json:"id"`
	Run     string   `json:"run"`
	Paths   []string `json:"paths,omitempty"`
	Needs   []string `json:"needs,omitempty"`
	Setup   bool     `json:"setup,omitempty"`
	Creates string   `json:"creates,omitempty"`
}

type ChartRenderPayload struct {
	Chart string `json:"chart"`
}

type SubagentPayload struct {
	AgentID   string `json:"agent_id"`
	AgentType string `json:"agent_type"`
}
