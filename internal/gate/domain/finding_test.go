package domain_test

import (
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/gate/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func TestBaselineNewFindings(t *testing.T) {
	known := shareddomain.Finding{Rule: "no-any", Severity: shareddomain.SeverityError, File: "a.ts", Line: 10, Message: "any is banned"}
	movedKnown := known
	movedKnown.Line = 42
	newRule := shareddomain.Finding{Rule: "no-eval", Severity: shareddomain.SeverityError, File: "a.ts", Line: 10, Message: "any is banned"}
	newFile := shareddomain.Finding{Rule: "no-any", Severity: shareddomain.SeverityError, File: "b.ts", Line: 10, Message: "any is banned"}
	newMessage := shareddomain.Finding{Rule: "no-any", Severity: shareddomain.SeverityError, File: "a.ts", Line: 10, Message: "other text"}
	warning := shareddomain.Finding{Rule: "style", Severity: shareddomain.SeverityWarn, File: "c.ts", Line: 1, Message: "style"}

	cases := []struct {
		name     string
		found    []shareddomain.Finding
		baseline []shareddomain.Finding
		want     []shareddomain.Finding
	}{
		{name: "moved line is not new", found: []shareddomain.Finding{movedKnown}, baseline: []shareddomain.Finding{known}},
		{name: "empty baseline keeps every error", found: []shareddomain.Finding{known, newFile}, want: []shareddomain.Finding{known, newFile}},
		{name: "warning is never new", found: []shareddomain.Finding{warning}},
		{
			name:     "other rule, file or message is new",
			found:    []shareddomain.Finding{movedKnown, newRule, newFile, newMessage, warning},
			baseline: []shareddomain.Finding{known},
			want:     []shareddomain.Finding{newRule, newFile, newMessage},
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
