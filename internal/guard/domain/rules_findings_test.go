package domain

import (
	"strings"
	"testing"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func finding(id string, severity shareddomain.ReviewSeverity, verdict shareddomain.FindingVerdict, status shareddomain.FindingStatus) shareddomain.ReviewFinding {
	return shareddomain.ReviewFinding{ID: id, Severity: severity, Verdict: verdict, Status: status}
}

func TestReviewFindingsShipDomain(t *testing.T) {
	real, open := shareddomain.FindingReal, shareddomain.FindingOpen
	cases := []struct {
		name     string
		findings []shareddomain.ReviewFinding
		strict   bool
		want     []string
	}{
		{"blocker open real", []shareddomain.ReviewFinding{finding("F1", shareddomain.SeverityBlocker, real, open)}, false, []string{"F1"}},
		{"major open real", []shareddomain.ReviewFinding{finding("F2", shareddomain.SeverityMajor, real, open)}, false, []string{"F2"}},
		{"minor open real", []shareddomain.ReviewFinding{finding("F3", shareddomain.SeverityMinor, real, open)}, false, nil},
		{"minor open real strict", []shareddomain.ReviewFinding{finding("F3", shareddomain.SeverityMinor, real, open)}, true, []string{"F3"}},
		{"not real", []shareddomain.ReviewFinding{finding("F4", shareddomain.SeverityBlocker, shareddomain.FindingNotReal, open)}, true, nil},
		{"closed", []shareddomain.ReviewFinding{finding("F5", shareddomain.SeverityBlocker, real, shareddomain.FindingClosed)}, true, nil},
		{"none", nil, true, nil},
		{"two named", []shareddomain.ReviewFinding{
			finding("F6", shareddomain.SeverityBlocker, real, open),
			finding("F7", shareddomain.SeverityMajor, real, open),
		}, false, []string{"F6", "F7"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, denied := DenyOpenFindings(c.findings, c.strict)
			if denied != (len(c.want) > 0) {
				t.Fatalf("denied = %v (%q), want %v", denied, reason, len(c.want) > 0)
			}
			if !denied {
				return
			}
			for _, part := range append(c.want, nextStepFindings) {
				if !strings.Contains(reason, part) {
					t.Errorf("reason %q does not contain %q", reason, part)
				}
			}
		})
	}
}

func TestReviewFindingsNeedRework(t *testing.T) {
	real, notReal := shareddomain.FindingReal, shareddomain.FindingNotReal
	open, closed := shareddomain.FindingOpen, shareddomain.FindingClosed
	cases := []struct {
		name     string
		findings []shareddomain.ReviewFinding
		reasons  []string
		want     []string
	}{
		{"closed major without rework", []shareddomain.ReviewFinding{finding("F1", shareddomain.SeverityMajor, real, closed)}, nil, []string{"F1"}},
		{"open blocker without rework", []shareddomain.ReviewFinding{finding("F1", shareddomain.SeverityBlocker, real, open)}, []string{"F2, major, other"}, []string{"F1"}},
		{"named in a reason", []shareddomain.ReviewFinding{finding("F1", shareddomain.SeverityMajor, real, closed)}, []string{"F1, major, fix the pong"}, nil},
		{"id is a prefix of another", []shareddomain.ReviewFinding{finding("F1", shareddomain.SeverityMajor, real, closed)}, []string{"F10, major, other"}, []string{"F1"}},
		{"minor", []shareddomain.ReviewFinding{finding("F1", shareddomain.SeverityMinor, real, closed)}, nil, nil},
		{"not real", []shareddomain.ReviewFinding{finding("F1", shareddomain.SeverityMajor, notReal, closed)}, nil, nil},
		{"two unreworked", []shareddomain.ReviewFinding{
			finding("F1", shareddomain.SeverityMajor, real, closed),
			finding("F2", shareddomain.SeverityBlocker, real, closed),
			finding("F3", shareddomain.SeverityMajor, real, closed),
		}, []string{"F3, major, fix"}, []string{"F1", "F2"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reason, denied := DenyUnreworkedFindings(c.findings, c.reasons)
			if denied != (len(c.want) > 0) {
				t.Fatalf("denied = %v (%q), want %v", denied, reason, len(c.want) > 0)
			}
			for _, part := range append(c.want, nextStepRework) {
				if denied && !strings.Contains(reason, part) {
					t.Errorf("reason %q does not contain %q", reason, part)
				}
			}
		})
	}
}
