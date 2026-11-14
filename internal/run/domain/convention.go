package domain

import (
	"context"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type CandidateSource string

const (
	SourceStandards     CandidateSource = "standards"
	SourceGateCheckable CandidateSource = "gate-checkable"
	SourceNoSeam        CandidateSource = "no-seam"
)

const (
	lensStandards  = "standards"
	noSeamID       = "reproduction"
	noSeamLocation = "plan.md"
)

// NoSeamReader gives the reason that a run's plan records for a missing regression-test seam.
// It returns an empty reason when the plan records none or when the run has no plan file.
type NoSeamReader interface {
	NoSeam(ctx context.Context, run Run) (string, error)
}

// NoSeamCandidate turns the reason of a missing seam into the first learn candidate. A blank reason gives none.
func NoSeamCandidate(reason string) (Candidate, bool) {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return Candidate{}, false
	}
	return Candidate{Source: SourceNoSeam, ID: noSeamID, Location: noSeamLocation, Text: reason}, true
}

type Candidate struct {
	Source   CandidateSource
	ID       string
	Location string
	Text     string
}

// ConventionCandidates lists the real findings that hint at a durable rule, in the order of findings.md.
// A finding counts when its lens is standards, or when a gate-checkable rework names it. Standards wins.
func ConventionCandidates(findings []shareddomain.ReviewFinding, transitions []Transitioned) []Candidate {
	var reasons []string
	for _, t := range transitions {
		if t.Event.IsRework() && t.Class == ReworkGateCheckable {
			reasons = append(reasons, t.Reason)
		}
	}
	var candidates []Candidate
	for _, f := range findings {
		if f.Verdict != shareddomain.FindingReal {
			continue
		}
		source, ok := candidateSource(f, reasons)
		if !ok {
			continue
		}
		candidates = append(candidates, Candidate{Source: source, ID: f.ID, Location: f.Location, Text: f.Text})
	}
	return candidates
}

func candidateSource(f shareddomain.ReviewFinding, gateReasons []string) (CandidateSource, bool) {
	if f.Lens == lensStandards {
		return SourceStandards, true
	}
	for _, reason := range gateReasons {
		if shareddomain.NamesFinding(reason, f.ID) {
			return SourceGateCheckable, true
		}
	}
	return "", false
}
