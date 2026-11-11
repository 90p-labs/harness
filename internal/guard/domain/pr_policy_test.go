package domain

import (
	"slices"
	"testing"
)

func TestPRPolicy(t *testing.T) {
	cases := []struct {
		name     string
		disabled []string
		want     PRPolicy
	}{
		{name: "none disabled", disabled: nil, want: PRPolicy{Draft: DraftRequired, Merge: MergeUser}},
		{name: "draft off", disabled: []string{RulePRDraftOnly}, want: PRPolicy{Draft: DraftOptional, Merge: MergeUser}},
		{name: "merge off", disabled: []string{RulePRNoMerge}, want: PRPolicy{Draft: DraftRequired, Merge: MergeAgent}},
		{name: "both off", disabled: []string{RulePRNoMerge, RulePRDraftOnly}, want: PRPolicy{Draft: DraftOptional, Merge: MergeAgent}},
		{name: "other rule off", disabled: []string{RuleStopHoldsTurn}, want: PRPolicy{Draft: DraftRequired, Merge: MergeUser}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := PRPolicyOf(tc.disabled)
			if err != nil || got != tc.want {
				t.Fatalf("PRPolicyOf(%v) = %+v, %v, want %+v", tc.disabled, got, err, tc.want)
			}
		})
	}

	for _, id := range []string{"bogus", "PR-NO-MERGE", " pr-no-merge", ""} {
		t.Run("unknown "+id, func(t *testing.T) {
			if _, err := PRPolicyOf([]string{RulePRDraftOnly, id}); err == nil {
				t.Fatalf("PRPolicyOf with %q: err = nil, want an error", id)
			}
		})
	}

	t.Run("check rule ids", func(t *testing.T) {
		if err := CheckRuleIDs([]string{RulePRDraftOnly, RuleStopHoldsTurn}); err != nil {
			t.Fatalf("CheckRuleIDs(known ids) = %v, want nil", err)
		}
		if err := CheckRuleIDs(nil); err != nil {
			t.Fatalf("CheckRuleIDs(nil) = %v, want nil", err)
		}
		err := CheckRuleIDs([]string{RulePRDraftOnly, "bogus"})
		if want := `guard.disabled names unknown rule "bogus"`; err == nil || err.Error() != want {
			t.Fatalf("CheckRuleIDs(bogus) = %v, want %q", err, want)
		}
	})

	t.Run("known ids", func(t *testing.T) {
		known := KnownRuleIDs()
		for _, rule := range Rules() {
			if !slices.Contains(known, rule.ID) {
				t.Errorf("KnownRuleIDs() = %v, missing %q", known, rule.ID)
			}
		}
		for _, id := range []string{RuleStopHoldsTurn, RuleReviewModels, RuleReviewFindings} {
			if !slices.Contains(known, id) {
				t.Errorf("KnownRuleIDs() = %v, missing %q", known, id)
			}
		}
	})
}
