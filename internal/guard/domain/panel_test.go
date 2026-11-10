package domain

import (
	"slices"
	"strings"
	"testing"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

func realFinding(id string, severity shareddomain.ReviewSeverity, model string) shareddomain.ReviewFinding {
	return shareddomain.ReviewFinding{
		ID: id, Severity: severity, Verdict: shareddomain.FindingReal, Status: shareddomain.FindingClosed, Model: model,
	}
}

func TestPanelModels(t *testing.T) {
	listed := []string{"haiku", "sonnet", "opus", "fable"}
	rework := ReworkEvent{Name: shareddomain.TransitionRequestRework, Reason: "fix F1 and F3"}
	findings := []shareddomain.ReviewFinding{
		realFinding("F1", shareddomain.SeverityBlocker, "opus"),
		realFinding("F2", shareddomain.SeverityMajor, "fable"),
		realFinding("F3", shareddomain.SeverityMajor, "haiku"),
		realFinding("F10", shareddomain.SeverityMajor, "fable"),
	}
	small := PanelFacts{Listed: listed, Findings: findings, Last: rework, Fixup: FixupSize{Lines: 10, Known: true}, Limit: 80}
	with := func(edit func(f *PanelFacts)) PanelFacts {
		f := small
		f.Findings = slices.Clone(findings)
		edit(&f)
		return f
	}
	cases := []struct {
		name       string
		facts      PanelFacts
		want       []string
		wantReason string
	}{
		{"small panel adds the models of the reworked findings", small, []string{"haiku", "sonnet", "opus"}, "small panel"},
		{"whole word match keeps F1 apart from F10", with(func(f *PanelFacts) { f.Last.Reason = "fix F10" }), []string{"haiku", "sonnet", "fable"}, "small panel"},
		{"fixup at the limit stays small", with(func(f *PanelFacts) { f.Fixup.Lines = 80 }), []string{"haiku", "sonnet", "opus"}, "small panel"},
		{"two listed models are the panel", with(func(f *PanelFacts) {
			f.Listed = []string{"haiku", "sonnet"}
			f.Findings[0].Model = "sonnet"
		}), []string{"haiku", "sonnet"}, "small panel"},
		{"no rework", with(func(f *PanelFacts) { f.Last = ReworkEvent{} }), listed, "no rework"},
		{"last amend", with(func(f *PanelFacts) { f.Last.Name = shareddomain.TransitionAmend }), listed, "amend"},
		{"last regress", with(func(f *PanelFacts) { f.Last.Name = shareddomain.TransitionRegress }), listed, "regress"},
		{"last rework-minor", with(func(f *PanelFacts) { f.Last.Name = shareddomain.TransitionReworkMinor }), listed, "rework-minor"},
		{"large fixup", with(func(f *PanelFacts) { f.Fixup.Lines = 81 }), listed, "fixup"},
		{"unknown fixup", with(func(f *PanelFacts) { f.Fixup = FixupSize{} }), listed, "fixup"},
		{"no Model cell", with(func(f *PanelFacts) { f.Findings[0].Model = "" }), listed, "Model"},
		{"unlisted model", with(func(f *PanelFacts) { f.Findings[0].Model = "gpt-9" }), listed, "Model"},
		{"reason names no ID", with(func(f *PanelFacts) { f.Last.Reason = "polish" }), listed, "no finding"},
		{"a not real finding is not reworked", with(func(f *PanelFacts) { f.Findings[0].Verdict = shareddomain.FindingNotReal }), []string{"haiku", "sonnet"}, "small panel"},
		{"reason names a minor finding only", with(func(f *PanelFacts) {
			f.Findings = []shareddomain.ReviewFinding{realFinding("F1", shareddomain.SeverityMinor, "opus")}
			f.Last.Reason = "fix F1"
		}), listed, "no finding"},
		{"no Model column", with(func(f *PanelFacts) {
			for i := range f.Findings {
				f.Findings[i].Model = ""
			}
		}), listed, "Model"},
		{"no findings", with(func(f *PanelFacts) { f.Findings = nil }), listed, "no finding"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := PanelModels(tc.facts)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("models = %v, want %v (%s)", got, tc.want, reason)
			}
			if !strings.Contains(reason, tc.wantReason) {
				t.Fatalf("reason = %q, want it to contain %q", reason, tc.wantReason)
			}
		})
	}
}
