package domain_test

import (
	"reflect"
	"testing"

	"github.com/jorge07/harness/internal/gate/domain"
)

func TestBaselineNewFindings(t *testing.T) {
	known := domain.Finding{Rule: "no-any", Severity: domain.SeverityError, File: "a.ts", Line: 10, Message: "any is banned"}
	movedKnown := known
	movedKnown.Line = 42
	newRule := domain.Finding{Rule: "no-eval", Severity: domain.SeverityError, File: "a.ts", Line: 10, Message: "any is banned"}
	newFile := domain.Finding{Rule: "no-any", Severity: domain.SeverityError, File: "b.ts", Line: 10, Message: "any is banned"}
	newMessage := domain.Finding{Rule: "no-any", Severity: domain.SeverityError, File: "a.ts", Line: 10, Message: "other text"}
	warning := domain.Finding{Rule: "style", Severity: "warning", File: "c.ts", Line: 1, Message: "style"}

	cases := []struct {
		name     string
		found    []domain.Finding
		baseline []domain.Finding
		want     []domain.Finding
	}{
		{name: "moved line is not new", found: []domain.Finding{movedKnown}, baseline: []domain.Finding{known}},
		{name: "empty baseline keeps every error", found: []domain.Finding{known, newFile}, want: []domain.Finding{known, newFile}},
		{name: "warning is never new", found: []domain.Finding{warning}},
		{
			name:     "other rule, file or message is new",
			found:    []domain.Finding{movedKnown, newRule, newFile, newMessage, warning},
			baseline: []domain.Finding{known},
			want:     []domain.Finding{newRule, newFile, newMessage},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.NewFindings(tc.found, tc.baseline)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("new findings = %+v, want %+v", got, tc.want)
			}
		})
	}
}
