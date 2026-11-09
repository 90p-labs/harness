package domain_test

import (
	"testing"

	"github.com/jorge07/harness/internal/shared/domain"
)

func TestTransitionFilterMatches(t *testing.T) {
	payload := domain.TransitionPayload{Event: domain.TransitionStart, From: "", To: domain.PhasePlanning}
	cases := map[string]struct {
		filter domain.TransitionFilter
		want   bool
	}{
		"empty matches all":        {domain.TransitionFilter{}, true},
		"or within a list":         {domain.TransitionFilter{Transition: []string{domain.TransitionClose, domain.TransitionStart}}, true},
		"and across lists":         {domain.TransitionFilter{Transition: []string{domain.TransitionStart}, To: []string{domain.PhaseDone}}, false},
		"all lists match":          {domain.TransitionFilter{Transition: []string{domain.TransitionStart}, To: []string{domain.PhasePlanning}}, true},
		"from never matches empty": {domain.TransitionFilter{From: []string{domain.PhasePlanning}}, false},
		"to mismatch":              {domain.TransitionFilter{To: []string{domain.PhaseDone}}, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := tc.filter.Matches(payload); got != tc.want {
				t.Fatalf("Matches = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTransitionFilterEmpty(t *testing.T) {
	if !(domain.TransitionFilter{}).Empty() {
		t.Fatal("zero filter must be empty")
	}
	if (domain.TransitionFilter{To: []string{domain.PhaseDone}}).Empty() {
		t.Fatal("filter with a list must not be empty")
	}
}
