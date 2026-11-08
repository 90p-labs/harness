package domain_test

import (
	"testing"

	"github.com/jorge07/harness/internal/shared/domain"
)

func TestQuestionTools(t *testing.T) {
	want := map[domain.Host]domain.QuestionTool{
		domain.HostClaude: {Name: "AskUserQuestion", MaxQuestions: 4, MinOptions: 2, MaxOptions: 4},
		domain.HostCodex:  {Name: "request_user_input", MaxQuestions: 3, MinOptions: 2, MaxOptions: 3},
	}
	if len(domain.QuestionTools) != len(want) {
		t.Fatalf("QuestionTools has %d hosts, want %d", len(domain.QuestionTools), len(want))
	}
	for host, tool := range want {
		if got := domain.QuestionTools[host]; got != tool {
			t.Errorf("QuestionTools[%s] = %+v, want %+v", host, got, tool)
		}
	}
}

func TestQuestionToolsApprovesPlan(t *testing.T) {
	approve := domain.Answer{Header: domain.ApprovalHeader, Label: domain.ApproveLabel}
	cases := []struct {
		name    string
		answers []domain.Answer
		want    bool
	}{
		{"no answer", nil, false},
		{"approve", []domain.Answer{approve}, true},
		{"approve after another answer", []domain.Answer{{Header: "Scope", Label: "Yes"}, approve}, true},
		{"change the plan", []domain.Answer{{Header: domain.ApprovalHeader, Label: domain.ChangePlanLabel}}, false},
		{"approve label under another header", []domain.Answer{{Header: "Scope", Label: domain.ApproveLabel}}, false},
		{"typed text", []domain.Answer{{Header: domain.ApprovalHeader, Label: "approve it"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := domain.ApprovesPlan(c.answers); got != c.want {
				t.Fatalf("ApprovesPlan(%+v) = %v, want %v", c.answers, got, c.want)
			}
		})
	}
}
