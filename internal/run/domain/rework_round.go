package domain

import (
	"context"
	"fmt"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type FindingsReader interface {
	Read(ctx context.Context, run Run) ([]shareddomain.ReviewFinding, error)
}

func ReworkEvent(findings []shareddomain.ReviewFinding, policy Policy, reason string) (Event, error) {
	var open []shareddomain.ReviewFinding
	for _, finding := range findings {
		if finding.Verdict == shareddomain.FindingReal && finding.Status == shareddomain.FindingOpen {
			open = append(open, finding)
		}
	}
	if len(open) == 0 {
		return "", ErrNoOpenFinding
	}
	if err := namesBlockersAndMajors(open, reason); err != nil {
		return "", err
	}
	for _, finding := range open {
		if finding.Severity != shareddomain.SeverityMinor || policy.StrictMinor {
			return EventRequestRework, nil
		}
	}
	return EventReworkMinor, nil
}

func namesBlockersAndMajors(open []shareddomain.ReviewFinding, reason string) error {
	var missing []string
	for _, finding := range open {
		if finding.Severity != shareddomain.SeverityMinor && !shareddomain.NamesFinding(reason, finding.ID) {
			missing = append(missing, finding.ID)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: missing %s", ErrReasonNamesNoFinding, strings.Join(missing, ", "))
	}
	return nil
}
