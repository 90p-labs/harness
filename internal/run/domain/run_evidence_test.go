package domain_test

import (
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
)

func eventOf(event domain.Event, reason string) domain.Transitioned {
	return domain.Transitioned{RunID: "r1", Event: event, Reason: reason}
}

func TestRunEvidence(t *testing.T) {
	for name, tc := range map[string]struct {
		run         domain.Run
		transitions []domain.Transitioned
		want        []domain.RunEvidence
	}{
		"a request-rework with a class and a reason": {
			transitions: []domain.Transitioned{reworkOf(domain.EventRequestRework, domain.ReworkImplementation, "F1 major a.go:3")},
			want:        []domain.RunEvidence{{ID: "rework-1", Text: "implementation: F1 major a.go:3"}},
		},
		"a rework-minor with a class and a reason": {
			transitions: []domain.Transitioned{reworkOf(domain.EventReworkMinor, domain.ReworkMinor, "rename F2")},
			want:        []domain.RunEvidence{{ID: "rework-1", Text: "minor: rename F2"}},
		},
		"a rework with a class and no reason": {
			transitions: []domain.Transitioned{reworkOf(domain.EventRequestRework, domain.ReworkSpecGap, "")},
			want:        []domain.RunEvidence{{ID: "rework-1", Text: "spec-gap"}},
		},
		"a rework with a reason and no class": {
			transitions: []domain.Transitioned{reworkOf(domain.EventRequestRework, "", "fix F1")},
			want:        []domain.RunEvidence{{ID: "rework-1", Text: "fix F1"}},
		},
		"a rework with neither": {
			transitions: []domain.Transitioned{reworkOf(domain.EventRequestRework, "", "")},
			want:        []domain.RunEvidence{{ID: "rework-1", Text: ""}},
		},
		"a block with a reason": {
			transitions: []domain.Transitioned{eventOf(domain.EventBlock, "wait for the token")},
			want:        []domain.RunEvidence{{ID: "block-1", Text: "wait for the token"}},
		},
		"an amend counts": {
			transitions: []domain.Transitioned{eventOf(domain.EventAmend, ""), eventOf(domain.EventAmend, "")},
			want:        []domain.RunEvidence{{ID: "amends", Text: "2"}},
		},
		"the stop blocks of the run": {
			run:  domain.Run{StopBlocksTotal: 3},
			want: []domain.RunEvidence{{ID: "stop-blocks", Text: "3"}},
		},
		"a reason with a line break goes on one line": {
			transitions: []domain.Transitioned{
				reworkOf(domain.EventRequestRework, domain.ReworkImplementation, "F1 major\n  a.go:3\r\n\tname it"),
				eventOf(domain.EventBlock, "wait\nfor   the token\n"),
			},
			want: []domain.RunEvidence{
				{ID: "rework-1", Text: "implementation: F1 major a.go:3 name it"},
				{ID: "block-1", Text: "wait for the token"},
			},
		},
		"events outside the list give nothing": {
			transitions: []domain.Transitioned{
				eventOf(domain.EventResume, "back"), eventOf(domain.EventRegress, "broken"), eventOf(domain.EventSubmitPlan, ""),
			},
		},
		"the order is reworks and blocks in log order, then amends, then stop blocks": {
			run: domain.Run{StopBlocksTotal: 1},
			transitions: []domain.Transitioned{
				eventOf(domain.EventAmend, ""),
				eventOf(domain.EventBlock, "first block"),
				reworkOf(domain.EventRequestRework, domain.ReworkSpecGap, "first rework"),
				eventOf(domain.EventResume, ""),
				eventOf(domain.EventBlock, "second block"),
				reworkOf(domain.EventReworkMinor, domain.ReworkMinor, "second rework"),
			},
			want: []domain.RunEvidence{
				{ID: "block-1", Text: "first block"},
				{ID: "rework-1", Text: "spec-gap: first rework"},
				{ID: "block-2", Text: "second block"},
				{ID: "rework-2", Text: "minor: second rework"},
				{ID: "amends", Text: "1"},
				{ID: "stop-blocks", Text: "1"},
			},
		},
		"a run with no evidence": {
			transitions: []domain.Transitioned{eventOf(domain.EventStart, "")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := domain.ListRunEvidence(tc.run, tc.transitions)

			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
