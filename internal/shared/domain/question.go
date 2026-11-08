package domain

import "slices"

type QuestionTool struct {
	Name         string
	MaxQuestions int
	MinOptions   int
	MaxOptions   int
}

const (
	ApprovalHeader    = "Plan"
	ApproveLabel      = "Approve"
	ChangePlanLabel   = "Change the plan"
	RecommendedSuffix = " (Recommended)"
)

var QuestionTools = map[Host]QuestionTool{
	HostClaude: {Name: "AskUserQuestion", MaxQuestions: 4, MinOptions: 2, MaxOptions: 4},
	HostCodex:  {Name: "request_user_input", MaxQuestions: 3, MinOptions: 2, MaxOptions: 3},
}

type Answer struct {
	Header string `json:"header"`
	Label  string `json:"label"`
}

func ApprovesPlan(answers []Answer) bool {
	return slices.Contains(answers, Answer{Header: ApprovalHeader, Label: ApproveLabel})
}
