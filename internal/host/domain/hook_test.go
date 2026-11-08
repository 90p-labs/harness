package domain_test

import (
	"testing"

	"github.com/jorge07/harness/internal/host/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

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
