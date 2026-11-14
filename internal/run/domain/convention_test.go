package domain_test

import (
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func lensFinding(id, lens string, verdict shareddomain.FindingVerdict) shareddomain.ReviewFinding {
	f := finding(id, shareddomain.SeverityMajor)
	f.Lens, f.Verdict = lens, verdict
	f.Location, f.Text = id+".go:1", "text of "+id
	return f
}

func reworkOf(event domain.Event, class domain.ReworkClass, reason string) domain.Transitioned {
	return domain.Transitioned{
		RunID: "r1", From: domain.PhaseReviewing, Event: event, To: domain.PhaseWorking, Reason: reason, Class: class,
	}
}

func candidate(source domain.CandidateSource, id string) domain.Candidate {
	return domain.Candidate{Source: source, ID: id, Location: id + ".go:1", Text: "text of " + id}
}

func TestConventionCandidates(t *testing.T) {
	const lensStandards, lensSpec = "standards", "spec"
	real, notReal := shareddomain.FindingReal, shareddomain.FindingNotReal

	for name, tc := range map[string]struct {
		findings    []shareddomain.ReviewFinding
		transitions []domain.Transitioned
		want        []domain.Candidate
	}{
		"a real standards finding": {
			findings: []shareddomain.ReviewFinding{lensFinding("S1", lensStandards, real)},
			want:     []domain.Candidate{candidate(domain.SourceStandards, "S1")},
		},
		"a spec finding": {
			findings: []shareddomain.ReviewFinding{lensFinding("P1", lensSpec, real)},
		},
		"a real finding with no lens": {
			findings: []shareddomain.ReviewFinding{lensFinding("F1", "", real)},
		},
		"a not real standards finding": {
			findings: []shareddomain.ReviewFinding{lensFinding("S1", lensStandards, notReal)},
		},
		"a gate-checkable rework names a real finding": {
			findings:    []shareddomain.ReviewFinding{lensFinding("P1", lensSpec, real), lensFinding("F2", "", real)},
			transitions: []domain.Transitioned{reworkOf(domain.EventRequestRework, domain.ReworkGateCheckable, "fix P1, F2")},
			want: []domain.Candidate{
				candidate(domain.SourceGateCheckable, "P1"), candidate(domain.SourceGateCheckable, "F2"),
			},
		},
		"a gate-checkable minor rework counts": {
			findings:    []shareddomain.ReviewFinding{lensFinding("F1", "", real)},
			transitions: []domain.Transitioned{reworkOf(domain.EventReworkMinor, domain.ReworkGateCheckable, "fix F1")},
			want:        []domain.Candidate{candidate(domain.SourceGateCheckable, "F1")},
		},
		"a gate-checkable rework of a not real finding": {
			findings:    []shareddomain.ReviewFinding{lensFinding("F1", "", notReal)},
			transitions: []domain.Transitioned{reworkOf(domain.EventRequestRework, domain.ReworkGateCheckable, "fix F1")},
		},
		"other classes are one-off bugs": {
			findings: []shareddomain.ReviewFinding{lensFinding("F1", "", real), lensFinding("F2", "", real), lensFinding("F3", "", real)},
			transitions: []domain.Transitioned{
				reworkOf(domain.EventRequestRework, domain.ReworkSpecGap, "fix F1"),
				reworkOf(domain.EventRequestRework, domain.ReworkImplementation, "fix F2"),
				reworkOf(domain.EventReworkMinor, domain.ReworkMinor, "fix F3"),
			},
		},
		"a gate-checkable class on another event": {
			findings:    []shareddomain.ReviewFinding{lensFinding("F1", "", real)},
			transitions: []domain.Transitioned{reworkOf(domain.EventAmend, domain.ReworkGateCheckable, "fix F1")},
		},
		"the reason names a whole ID": {
			findings:    []shareddomain.ReviewFinding{lensFinding("F1", "", real)},
			transitions: []domain.Transitioned{reworkOf(domain.EventRequestRework, domain.ReworkGateCheckable, "fix F10")},
		},
		"standards wins and the finding is listed once": {
			findings: []shareddomain.ReviewFinding{lensFinding("S1", lensStandards, real)},
			transitions: []domain.Transitioned{
				reworkOf(domain.EventRequestRework, domain.ReworkGateCheckable, "fix S1"),
				reworkOf(domain.EventReworkMinor, domain.ReworkGateCheckable, "fix S1 again"),
			},
			want: []domain.Candidate{candidate(domain.SourceStandards, "S1")},
		},
		"the order of findings.md": {
			findings: []shareddomain.ReviewFinding{
				lensFinding("F3", "", real), lensFinding("S2", lensStandards, real), lensFinding("F1", "", real),
			},
			transitions: []domain.Transitioned{reworkOf(domain.EventRequestRework, domain.ReworkGateCheckable, "F1 F3")},
			want: []domain.Candidate{
				candidate(domain.SourceGateCheckable, "F3"), candidate(domain.SourceStandards, "S2"),
				candidate(domain.SourceGateCheckable, "F1"),
			},
		},
		"no finding": {},
	} {
		t.Run(name, func(t *testing.T) {
			got := domain.ConventionCandidates(tc.findings, tc.transitions)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestNoSeamCandidate(t *testing.T) {
	if domain.SourceNoSeam != "no-seam" {
		t.Fatalf("SourceNoSeam = %q", domain.SourceNoSeam)
	}
	for name, tc := range map[string]struct {
		reason string
		want   domain.Candidate
		ok     bool
	}{
		"a reason": {
			reason: "the cache has no seam",
			want:   domain.Candidate{Source: domain.SourceNoSeam, ID: "reproduction", Location: "plan.md", Text: "the cache has no seam"},
			ok:     true,
		},
		"a reason with spaces around": {
			reason: "  no seam  ",
			want:   domain.Candidate{Source: domain.SourceNoSeam, ID: "reproduction", Location: "plan.md", Text: "no seam"},
			ok:     true,
		},
		"no reason":    {reason: ""},
		"blank reason": {reason: "   "},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := domain.NoSeamCandidate(tc.reason)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("got %+v, %v, want %+v, %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}
