package domain

import (
	"testing"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestAgentRole(t *testing.T) {
	cases := []struct {
		agentType string
		want      shareddomain.Role
	}{
		{"general-purpose", shareddomain.RoleWorker},
		{"claude", shareddomain.RoleWorker},
		{"90p:worker", shareddomain.RoleWorker},
		{"90p:general-purpose", shareddomain.RoleWorker},
		{"Explore", shareddomain.RoleExplorer},
		{"90p:verifier", shareddomain.RoleExplorer},
		{"CLAUDE", shareddomain.RoleWorker},
		{"kodify:Code-Reviewer", shareddomain.RoleReviewer},
		{"Plan", ""},
	}
	for _, c := range cases {
		if got := agentRole(c.agentType); got != c.want {
			t.Errorf("agentRole(%q) = %q, want %q", c.agentType, got, c.want)
		}
	}
}
