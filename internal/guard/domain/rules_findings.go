package domain

import (
	"fmt"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	RuleReviewFindings = "review-findings"

	nextStepFindings = "fix each finding with a rework, or close it in findings.md after its re-review, then approve the review again"
	nextStepRework   = "run harness run rework with --findings that names each of them, fix them, then review again"
)

func DenyOpenFindings(findings []shareddomain.ReviewFinding, strictMinor bool) (string, bool) {
	var open []string
	for _, f := range findings {
		if f.Verdict == shareddomain.FindingReal && f.Status == shareddomain.FindingOpen && blocksShip(f.Severity, strictMinor) {
			open = append(open, f.ID)
		}
	}
	if len(open) == 0 {
		return "", false
	}
	return deny(fmt.Sprintf("the review has %d open real findings: %s", len(open), strings.Join(open, modelSeparator)), nextStepFindings)
}

func DenyUnreworkedFindings(findings []shareddomain.ReviewFinding, reworkReasons []string) (string, bool) {
	var unreworked []string
	for _, f := range findings {
		if f.Verdict == shareddomain.FindingReal && isBlockerOrMajor(f.Severity) && !namedByAny(reworkReasons, f.ID) {
			unreworked = append(unreworked, f.ID)
		}
	}
	if len(unreworked) == 0 {
		return "", false
	}
	return deny(fmt.Sprintf("the real blocker or major findings %s have no rework", strings.Join(unreworked, modelSeparator)), nextStepRework)
}

func namedByAny(reasons []string, id string) bool {
	return slices.ContainsFunc(reasons, func(reason string) bool { return shareddomain.NamesFinding(reason, id) })
}

func HasBlockerOrMajor(findings []shareddomain.ReviewFinding) bool {
	return slices.ContainsFunc(findings, func(f shareddomain.ReviewFinding) bool {
		return f.Verdict == shareddomain.FindingReal && isBlockerOrMajor(f.Severity)
	})
}

func isBlockerOrMajor(severity shareddomain.ReviewSeverity) bool {
	return severity != shareddomain.SeverityMinor
}

func blocksShip(severity shareddomain.ReviewSeverity, strictMinor bool) bool {
	return strictMinor || isBlockerOrMajor(severity)
}
