package domain

const (
	RuleHeadingMissing   RuleID = "heading-missing"
	RuleCriterionUnowned RuleID = "criterion-unowned"
	RuleDoDUnknown       RuleID = "dod-unknown"
	RuleTaskDepends      RuleID = "task-depends"
	RuleTreeUnowned      RuleID = "tree-unowned"
	RuleTaskUncommitted  RuleID = "task-uncommitted"
	RuleSentenceLength   RuleID = "sentence-length"
	RuleEmDash           RuleID = "em-dash"
	RuleSignalsMissing   RuleID = "signals-missing"
	RuleRegressionImpact RuleID = "regression-impact"
	RuleDiagram          RuleID = "diagram"
	RuleTraceUnlinked    RuleID = "trace-unlinked"
)
