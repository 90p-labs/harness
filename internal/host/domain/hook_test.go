package domain_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/host/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestToolKindOfMCPName(t *testing.T) {
	cases := []struct {
		plain string
		want  shareddomain.ToolKind
	}{
		{"Bash", shareddomain.ToolKindShell},
		{"Edit", shareddomain.ToolKindEdit},
		{"MultiEdit", shareddomain.ToolKindEdit},
		{"apply_patch", shareddomain.ToolKindEdit},
		{"Write", shareddomain.ToolKindWrite},
		{"Read", shareddomain.ToolKindRead},
		{"Grep", shareddomain.ToolKindSearch},
		{"Glob", shareddomain.ToolKindSearch},
		{"Agent", shareddomain.ToolKindAgent},
		{"AskUserQuestion", shareddomain.ToolKindQuestion},
		{"request_user_input", shareddomain.ToolKindQuestion},
	}
	for _, tc := range cases {
		for _, name := range []string{tc.plain, "mcp__srv__" + tc.plain, "mcp__crawl_ostrich__act_" + tc.plain} {
			if got := domain.ToolKindOf(name); got != tc.want {
				t.Errorf("ToolKindOf(%q) = %q, want %q", name, got, tc.want)
			}
		}
	}
	for _, name := range []string{"mcp__crawl_ostrich__life_Bash", "mcp__crawl_ostrich__act_AskUserQuestion", "mcp__codex__act_request_user_input"} {
		if got := domain.ToolKindOf(name); got == shareddomain.ToolKindOther {
			t.Errorf("ToolKindOf(%q) = %q, want a known kind", name, got)
		}
	}
	if got := domain.ToolKindOf("mcp__github__create_issue"); got != shareddomain.ToolKindOther {
		t.Errorf("ToolKindOf(mcp__github__create_issue) = %q, want %q", got, shareddomain.ToolKindOther)
	}
}

func TestQuestionDecodeToolKind(t *testing.T) {
	cases := map[string]shareddomain.ToolKind{
		shareddomain.QuestionTools[shareddomain.HostClaude].Name: shareddomain.ToolKindQuestion,
		shareddomain.QuestionTools[shareddomain.HostCodex].Name:  shareddomain.ToolKindQuestion,
		"Bash":            shareddomain.ToolKindShell,
		"WebFetch":        shareddomain.ToolKindOther,
		"askuserquestion": shareddomain.ToolKindOther,
	}
	for tool, want := range cases {
		if got := domain.ToolKindOf(tool); got != want {
			t.Errorf("ToolKindOf(%q) = %q, want %q", tool, got, want)
		}
	}
}

func TestClaudeMatcher(t *testing.T) {
	wantTools := "AskUserQuestion Agent Bash Edit Glob Grep MultiEdit Read Write"
	got := domain.MatchedTools()
	if !slices.IsSorted(got) || len(got) != len(strings.Fields(wantTools)) {
		t.Fatalf("MatchedTools() = %v, want sorted %s", got, wantTools)
	}
	for _, tool := range strings.Fields(wantTools) {
		if !slices.Contains(got, tool) {
			t.Errorf("MatchedTools() = %v, misses %s", got, tool)
		}
	}
	for _, tool := range []string{"apply_patch", "request_user_input"} {
		if slices.Contains(got, tool) {
			t.Errorf("MatchedTools() holds Codex tool %s", tool)
		}
	}

	matcher := regexp.MustCompile(domain.ClaudeMatcher())
	cases := map[string]bool{
		"Bash":                                true,
		"AskUserQuestion":                     true,
		"mcp__x__life_Bash":                   true,
		"mcp__x__act_AskUserQuestion":         true,
		"mcp__crawl_ostrich__pilot_MultiEdit": true,
		"WebFetch":                            false,
		"mcp__x__web_Artifact":                false,
		"BashOutput":                          false,
		"mcp__x__life_BashOutput":             false,
		"apply_patch":                         false,
	}
	for tool, want := range cases {
		if matcher.MatchString(tool) != want {
			t.Errorf("matcher on %q = %v, want %v", tool, !want, want)
		}
	}
}
