package domain

import (
	"fmt"
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const smallPanelSize = 2

// ReworkEvent is the last rework-family event of the run: request-rework, rework-minor, amend or regress.
// An empty Name means the run has no such event.
type ReworkEvent struct {
	Name   string
	Reason string
}

// FixupSize is the added plus deleted lines between the last HEAD that every listed model reviewed and HEAD.
// Known is false when the size could not be read.
type FixupSize struct {
	Lines int
	Known bool
}

// PanelFacts holds the facts that pick the final review panel.
// Listed is the reviewer models in role order.
type PanelFacts struct {
	Listed   []string
	Findings []shareddomain.ReviewFinding
	Last     ReworkEvent
	Fixup    FixupSize
	Limit    int
	// Light marks a light review: a light track with no guard, security or hook path.
	Light bool
}

// PanelModels picks the models that must review before a ship, with a short reason.
// Any missing or odd fact gives every listed model.
func PanelModels(facts PanelFacts) (models []string, reason string) {
	full := func(why string) ([]string, string) {
		return slices.Clone(facts.Listed), "full panel: " + why
	}
	if facts.Light && len(facts.Listed) > 0 {
		return slices.Clone(facts.Listed[:1]), "light panel: light track with no guard, security or hook path"
	}
	switch facts.Last.Name {
	case "":
		return full("the run has no rework")
	case shareddomain.TransitionRequestRework:
	default:
		return full(fmt.Sprintf("the last rework event is %s", facts.Last.Name))
	}
	if !facts.Fixup.Known {
		return full("the fixup size is unknown")
	}
	if facts.Fixup.Lines > facts.Limit {
		return full(fmt.Sprintf("the fixup is %d lines, over the limit of %d", facts.Fixup.Lines, facts.Limit))
	}
	reworked := reworkedFindings(facts.Findings, facts.Last.Reason)
	if len(reworked) == 0 {
		return full("the rework reason names no finding of blocker or major severity")
	}
	panel := slices.Clone(facts.Listed[:min(smallPanelSize, len(facts.Listed))])
	var raised []string
	for _, finding := range reworked {
		if finding.Model == "" {
			return full(fmt.Sprintf("finding %s has no Model cell", finding.ID))
		}
		at := slices.IndexFunc(facts.Listed, func(model string) bool { return strings.EqualFold(model, finding.Model) })
		if at < 0 {
			return full(fmt.Sprintf("the Model cell %q of finding %s is not a listed reviewer", finding.Model, finding.ID))
		}
		if !slices.Contains(panel, facts.Listed[at]) {
			panel = append(panel, facts.Listed[at])
			raised = append(raised, facts.Listed[at])
		}
	}
	slices.SortFunc(panel, func(a, b string) int {
		return slices.Index(facts.Listed, a) - slices.Index(facts.Listed, b)
	})
	reason = fmt.Sprintf("small panel: the first %d reviewer models and the fixup of %d lines", min(smallPanelSize, len(facts.Listed)), facts.Fixup.Lines)
	if len(raised) > 0 {
		reason += ", plus " + strings.Join(raised, modelSeparator) + " for the reworked findings"
	}
	return panel, reason
}

func reworkedFindings(findings []shareddomain.ReviewFinding, reason string) []shareddomain.ReviewFinding {
	var reworked []shareddomain.ReviewFinding
	for _, finding := range findings {
		isBlocking := finding.Severity == shareddomain.SeverityBlocker || finding.Severity == shareddomain.SeverityMajor
		if finding.Verdict == shareddomain.FindingReal && isBlocking && shareddomain.NamesFinding(reason, finding.ID) {
			reworked = append(reworked, finding)
		}
	}
	return reworked
}
