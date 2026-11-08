package domain

import (
	"fmt"
	"strings"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	agentTypeSeparator = ":"
	noModel            = "no model"
)

var agentRoles = map[string]shareddomain.Role{
	"explore":           shareddomain.RoleExplorer,
	"general-purpose":   shareddomain.RoleExplorer,
	"claude-code-guide": shareddomain.RoleExplorer,
	"worker":            shareddomain.RoleWorker,
	"reviewer":          shareddomain.RoleReviewer,
	"second-reviewer":   shareddomain.RoleReviewer,
	"code-reviewer":     shareddomain.RoleReviewer,
	"pr-reviewer":       shareddomain.RoleReviewer,
}

func agentRole(agentType string) shareddomain.Role {
	if i := strings.LastIndex(agentType, agentTypeSeparator); i >= 0 {
		agentType = agentType[i+1:]
	}
	return agentRoles[strings.ToLower(agentType)]
}

func denyAgentModel(call ToolCall, rc RuleContext) (string, bool) {
	role := agentRole(call.AgentType)
	want, ok := rc.AgentModels[string(role)]
	if role == "" || !ok || call.Model == want {
		return "", false
	}
	got := call.Model
	if got == "" {
		got = noModel
	}
	return deny(
		fmt.Sprintf("the %s role runs on the model %s, and this call has %s", role, want, got),
		fmt.Sprintf("set the model to %s", want),
	)
}
