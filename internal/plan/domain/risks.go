package domain

import (
	"regexp"
	"slices"
	"strings"
)

const (
	RisksNone         = "none"
	MaxRunTasks       = 6
	ThreatAttacker    = "Attacker"
	ThreatInputForms  = "Input forms"
	ThreatFailClosed  = "Fail closed"
	RiskTestLabel     = "Test:"
	RiskDecisionLabel = "Decision:"
	FormRiskLine      = "`- text. Test: `TestName`.` or `- text. Decision: text.`"

	FolderGuard    = "guard"
	FolderSecurity = "security"
)

var (
	ThreatLabels  = []string{ThreatAttacker, ThreatInputForms, ThreatFailClosed}
	ThreatFolders = []string{FolderGuard, FolderSecurity}
	riskTestName  = regexp.MustCompile(regexp.QuoteMeta(RiskTestLabel) + " `([^`]+)`")
	riskDecision  = regexp.MustCompile(regexp.QuoteMeta(RiskDecisionLabel) + `\s+\S`)
)

type Risk struct {
	Text string
	Line int
}

func checkReviewRisks(doc Document, add report) {
	heading := headingLine(doc, 2, HeadingReviewRisks)
	if heading == 0 {
		return
	}
	if len(doc.ReviewRisks) == 0 {
		add(heading, RuleRiskUnanchored, "write `- %s` or one risk on each line, as %s", RisksNone, FormRiskLine)
		return
	}
	for _, r := range doc.ReviewRisks {
		if r.Text == RisksNone {
			continue
		}
		names := riskTestName.FindAllStringSubmatch(r.Text, -1)
		if len(names) == 0 && !riskDecision.MatchString(r.Text) {
			add(r.Line, RuleRiskUnanchored, "risk has neither a %s with a test name in backticks nor a %s with text", RiskTestLabel, RiskDecisionLabel)
		}
		for _, m := range names {
			if !runsTest(doc, m[1]) {
				add(r.Line, RuleRiskUnanchored, "test %s is in no criterion proof and in no Test first line", m[1])
			}
		}
	}
}

func runsTest(doc Document, name string) bool {
	word := regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`)
	return slices.ContainsFunc(doc.Criteria, func(c Criterion) bool { return word.MatchString(c.Proof) }) ||
		slices.ContainsFunc(doc.Tasks, func(t Task) bool { return word.MatchString(t.TestFirst) })
}

func hasThreatFolder(tree Tree) bool {
	return slices.ContainsFunc(tree.Entries, func(e TreeEntry) bool {
		if e.IsDir {
			return false
		}
		segments := strings.Split(e.Path, "/")
		return slices.ContainsFunc(segments[:len(segments)-1], func(s string) bool { return slices.Contains(ThreatFolders, s) })
	})
}

func checkThreatModel(doc Document, add report) {
	heading := headingLine(doc, 2, HeadingThreatModel)
	if heading == 0 {
		if hasThreatFolder(doc.CodeTree) || hasThreatFolder(doc.TestTree) {
			add(lineOr(headingLine(doc, 3, HeadingCodeTree), 1), RuleThreatModel,
				"a tree path has a %s folder, so the plan needs ## %s", strings.Join(ThreatFolders, " or "), HeadingThreatModel)
		}
		return
	}
	for _, label := range ThreatLabels {
		if !slices.ContainsFunc(doc.ThreatModel, func(b string) bool { return hasLabelText(b, label) }) {
			add(heading, RuleThreatModel, "## %s needs the bullet `- %s: text`", HeadingThreatModel, label)
		}
	}
}

func hasLabelText(bullet, label string) bool {
	text, ok := strings.CutPrefix(bullet, label+":")
	return ok && strings.TrimSpace(text) != ""
}

func checkPlanSize(doc Document, add report) {
	if len(doc.Tasks) > MaxRunTasks {
		add(lineOr(headingLine(doc, 2, HeadingTasks), 1), RulePlanSize,
			"the plan has %d tasks, more than %d: split the change into a multi-run plan with ## %s", len(doc.Tasks), MaxRunTasks, HeadingRuns)
	}
}
