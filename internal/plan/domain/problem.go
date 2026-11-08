package domain

import "fmt"

const PlanFile = "plan.md"

type RuleID string

const (
	RuleCriteriaTable RuleID = "criteria-table"
	RuleTaskField     RuleID = "task-field"
	RuleTreeFormat    RuleID = "tree-format"
	RuleCommitFormat  RuleID = "commit-format"
	RulePRSequence    RuleID = "pr-sequence"
	RuleSignalsTable  RuleID = "signals-table"
	RuleSpecCriteria  RuleID = "spec-criteria"
)

type Problem struct {
	Line    int
	Rule    RuleID
	Message string
}

func (p Problem) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", PlanFile, p.Line, p.Rule, p.Message)
}
