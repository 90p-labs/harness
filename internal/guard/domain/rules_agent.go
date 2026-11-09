package domain

import (
	"fmt"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	agentTypeSeparator = ":"
	noModel            = "no model"
	modelSeparator     = ", "
)

var agentRoles = map[string]shareddomain.Role{
	"explore":           shareddomain.RoleExplorer,
	"explorer":          shareddomain.RoleExplorer,
	"claude-code-guide": shareddomain.RoleExplorer,
	"general-purpose":   shareddomain.RoleWorker,
	"claude":            shareddomain.RoleWorker,
	"worker":            shareddomain.RoleWorker,
	"reviewer":          shareddomain.RoleReviewer,
	"second-reviewer":   shareddomain.RoleReviewer,
	"code-reviewer":     shareddomain.RoleReviewer,
	"pr-reviewer":       shareddomain.RoleReviewer,
	"advisor":           shareddomain.RoleAdvisor,
}

func agentRole(agentType string) shareddomain.Role {
	if i := strings.LastIndex(agentType, agentTypeSeparator); i >= 0 {
		agentType = agentType[i+1:]
	}
	return agentRoles[strings.ToLower(agentType)]
}

func denyAgentModel(call ToolCall, rc RuleContext) (string, bool) {
	role := agentRole(call.AgentType)
	models := rc.AgentModels[string(role)]
	if role == "" || len(models) == 0 || slices.Contains(models, call.Model) || (rc.ModelOptional && call.Model == "") {
		return "", false
	}
	got := call.Model
	if got == "" {
		got = noModel
	}
	want := strings.Join(models, modelSeparator)
	return deny(
		fmt.Sprintf("the %s role runs on one of the models %s, and this call has %s", role, want, got),
		fmt.Sprintf("set the model to one of %s", want),
	)
}
