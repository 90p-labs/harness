package cli_test

import (
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/run/application/query/status"
	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/run/ui/cli"
)

func TestStatusLines(t *testing.T) {
	working := status.Status{
		ID: "20261108-011805-add-the-login-page", Phase: domain.PhaseWorking,
		ReviewRound: 1, LiveRound: 0,
	}
	withReason := working
	withReason.Reason = "the tests miss the empty case"

	cases := []struct {
		name string
		in   status.Status
		want []string
	}{
		{"no reason", working, []string{
			"run 20261108-011805-add-the-login-page phase=working review_round=1 live_round=0",
			"next: implement test first, run harness verify, then harness run review",
		}},
		{"reason", withReason, []string{
			"run 20261108-011805-add-the-login-page phase=working review_round=1 live_round=0",
			"reason: the tests miss the empty case",
			"next: implement test first, run harness verify, then harness run review",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cli.Lines(tc.in)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("Lines = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStatusLinesHintForEachPhase(t *testing.T) {
	phases := []domain.Phase{
		domain.PhasePlanning, domain.PhaseAwaitingApproval, domain.PhaseWorking, domain.PhaseReviewing,
		domain.PhaseShipping, domain.PhaseReleasing, domain.PhaseObserving, domain.PhaseBlocked,
		domain.PhaseDone, domain.PhaseAbandoned,
	}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			lines := cli.Lines(status.Status{ID: "r1", Phase: phase})
			last := lines[len(lines)-1]
			if !strings.HasPrefix(last, "next: ") || strings.TrimPrefix(last, "next: ") == "" {
				t.Errorf("last line = %q, want a next hint", last)
			}
		})
	}
}
