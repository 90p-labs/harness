package domain

import (
	"fmt"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	RuleReviewFindings = "review-findings"

	nextStepFindings = "fix each finding with a rework, or close it in findings.md after its re-review, then approve the review again"
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

func blocksShip(severity shareddomain.ReviewSeverity, strictMinor bool) bool {
	return severity != shareddomain.SeverityMinor || strictMinor
}
