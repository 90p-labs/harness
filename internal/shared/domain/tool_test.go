package domain_test

import (
	"testing"

	"github.com/90p-labs/harness/internal/shared/domain"
)

func TestToolBaseName(t *testing.T) {
	known := map[string]bool{
		"Bash": true, "Edit": true, "MultiEdit": true, "Write": true, "Read": true,
		"Grep": true, "Glob": true, "Agent": true, "apply_patch": true,
		"AskUserQuestion": true, "request_user_input": true,
	}
	isKnown := func(name string) bool { return known[name] }

	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain Bash", "Bash", "Bash"},
		{"plain apply_patch", "apply_patch", "apply_patch"},
		{"plain request_user_input", "request_user_input", "request_user_input"},
		{"prefix word Bash", "mcp__crawl_ostrich__life_Bash", "Bash"},
		{"prefix word question", "mcp__crawl_ostrich__act_AskUserQuestion", "AskUserQuestion"},
		{"prefix word Edit", "mcp__crawl_ostrich__pilot_Edit", "Edit"},
		{"segment Bash", "mcp__srv__Bash", "Bash"},
		{"segment apply_patch", "mcp__srv__apply_patch", "apply_patch"},
		{"prefix word apply_patch", "mcp__srv__x_apply_patch", "apply_patch"},
		{"segment request_user_input", "mcp__srv__request_user_input", "request_user_input"},
		{"codex prefix word", "mcp__codex__act_request_user_input", "request_user_input"},

		{"no mcp prefix", "life_Bash", "life_Bash"},
		{"extra segments", "mcp__x__foo__Bash__y", "mcp__x__foo__Bash__y"},
		{"extra server segment", "mcp__x__foo__Bash", "mcp__x__foo__Bash"},
		{"empty server", "mcp____Bash", "mcp____Bash"},
		{"capital in word", "mcp__srv__Life_Bash", "mcp__srv__Life_Bash"},
		{"digit in word", "mcp__srv__l1fe_Bash", "mcp__srv__l1fe_Bash"},
		{"lowercase name", "mcp__srv__bash", "mcp__srv__bash"},
		{"suffix on name", "mcp__srv__life_BashX", "mcp__srv__life_BashX"},
		{"prefix on name", "NotBash", "NotBash"},
		{"unrelated tool", "mcp__github__create_issue", "mcp__github__create_issue"},
		{"empty segment", "mcp__srv__", "mcp__srv__"},
		{"server ending in underscore", "mcp__srv___Bash", "Bash"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := domain.ToolBaseName(tt.in, isKnown); got != tt.want {
				t.Fatalf("ToolBaseName(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
