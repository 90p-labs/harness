package domain_test

import (
	"testing"

	"github.com/jorge07/harness/internal/shared/domain"
)

func TestSeverityKnown(t *testing.T) {
	for _, severity := range []domain.Severity{domain.SeverityError, domain.SeverityWarn, domain.SeverityInfo} {
		if !severity.Known() {
			t.Errorf("%q is not known", severity)
		}
	}
	for _, severity := range []domain.Severity{"", "warning", "Error"} {
		if severity.Known() {
			t.Errorf("%q is known", severity)
		}
	}
}
