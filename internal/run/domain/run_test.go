package domain_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/jorge07/harness/internal/run/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

func apply(t *testing.T, run domain.Run, policy domain.Policy, event domain.Event, reason string) domain.Run {
	t.Helper()
	next, err := run.Apply(domain.Command{Event: event, Reason: reason}, policy)
	if err != nil {
		t.Fatalf("%s from %s: %v", event, run.Phase, err)
	}
	return next
}

func TestRunApply(t *testing.T) {
	old := domain.Run{ID: "r1"}
	next, err := old.Apply(domain.Command{Event: domain.EventStart}, domain.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if next.Phase != domain.PhasePlanning || next.ID != "r1" {
		t.Fatalf("got %+v", next)
	}
	if old.Phase != domain.PhaseNone {
		t.Fatalf("old run changed: %+v", old)
	}

	_, err = next.Apply(domain.Command{Event: domain.EventResume}, domain.Policy{})
	if !errors.Is(err, domain.ErrIllegalTransition) {
		t.Fatalf("got %v", err)
	}
}

func TestApplyRecordsTransition(t *testing.T) {
	at := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	run := domain.Run{ID: "r1", Version: 3}

	next, err := run.Apply(domain.Command{Event: domain.EventStart, Reason: "go", At: at}, domain.Policy{})
	if err != nil {
		t.Fatal(err)
	}

	want := []domain.Transitioned{{RunID: "r1", From: domain.PhaseNone, Event: domain.EventStart, To: domain.PhasePlanning, Reason: "go", At: at}}
	if !reflect.DeepEqual(next.Events(), want) {
		t.Fatalf("events = %+v, want %+v", next.Events(), want)
	}
	if next.Version != 3 || run.Events() != nil {
		t.Fatalf("version = %d, old events = %+v", next.Version, run.Events())
	}

	t.Run("a persisted run has the next version and no events", func(t *testing.T) {
		saved := next.Persisted()
		if saved.Version != 4 || saved.Events() != nil || len(next.Events()) != 1 {
			t.Fatalf("saved = %+v, events = %+v", saved, saved.Events())
		}
	})

	t.Run("two branches of one run share no events", func(t *testing.T) {
		base := domain.Run{ID: "r2"}
		for _, event := range []domain.Event{domain.EventStart, domain.EventSubmitPlan, domain.EventSubmitForReview} {
			base = apply(t, base, domain.Policy{}, event, "")
		}
		left := apply(t, base, domain.Policy{}, domain.EventBlock, "left")
		right := apply(t, base, domain.Policy{}, domain.EventAbandon, "right")

		if got := left.Events()[3].Reason; got != "left" {
			t.Fatalf("left event reason = %q, want left", got)
		}
		if got := right.Events()[3].Reason; got != "right" {
			t.Fatalf("right event reason = %q, want right", got)
		}
	})
}

func TestRunTrackNeverLowers(t *testing.T) {
	cases := []struct {
		name  string
		event domain.Event
		phase domain.Phase
		run   shareddomain.Track
		cmd   shareddomain.Track
		want  shareddomain.Track
	}{
		{"submit sets the first track", domain.EventSubmitPlan, domain.PhasePlanning, "", shareddomain.TrackLight, shareddomain.TrackLight},
		{"submit raises light to full", domain.EventSubmitPlan, domain.PhasePlanning, shareddomain.TrackLight, shareddomain.TrackFull, shareddomain.TrackFull},
		{"amend keeps full against light", domain.EventAmend, domain.PhaseWorking, shareddomain.TrackFull, shareddomain.TrackLight, shareddomain.TrackFull},
		{"amend keeps full against empty", domain.EventAmend, domain.PhaseWorking, shareddomain.TrackFull, "", shareddomain.TrackFull},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := domain.Run{Phase: c.phase, Track: c.run}
			next, err := run.Apply(domain.Command{Event: c.event, Track: c.cmd}, domain.Policy{})
			if err != nil {
				t.Fatal(err)
			}
			if next.Track != c.want {
				t.Fatalf("track = %q, want %q", next.Track, c.want)
			}
		})
	}

	t.Run("other events keep the track", func(t *testing.T) {
		run := domain.Run{Phase: domain.PhaseWorking, Track: shareddomain.TrackFull}
		next := apply(t, run, domain.Policy{}, domain.EventBlock, "stuck")
		if next.Track != shareddomain.TrackFull {
			t.Fatalf("track = %q", next.Track)
		}
	})
}

func TestApprovalGuard(t *testing.T) {
	cases := []struct {
		name   string
		policy domain.Policy
		live   int
		want   domain.Phase
	}{
		{"policy needs approval", domain.Policy{ApprovalNeeded: true}, 0, domain.PhaseAwaitingApproval},
		{"replan after regression", domain.Policy{}, 1, domain.PhaseAwaitingApproval},
		{"no approval", domain.Policy{}, 0, domain.PhaseWorking},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			run := domain.Run{Phase: domain.PhasePlanning, LiveRound: c.live}
			if got := apply(t, run, c.policy, domain.EventSubmitPlan, "").Phase; got != c.want {
				t.Fatalf("got %s want %s", got, c.want)
			}
		})
	}
}

func TestReworkRounds(t *testing.T) {
	policy := domain.Policy{MaxReviewRounds: 3}
	run := domain.Run{Phase: domain.PhaseReviewing, ReviewRound: 1}

	again := apply(t, run, policy, domain.EventRequestRework, "fix a")
	if again.Phase != domain.PhaseWorking || again.ReviewRound != 2 || again.Reason != "fix a" {
		t.Fatalf("got %+v", again)
	}

	last := apply(t, domain.Run{Phase: domain.PhaseReviewing, ReviewRound: 2}, policy, domain.EventRequestRework, "fix b")
	if last.Phase != domain.PhaseBlocked || last.BlockedFrom != domain.PhaseReviewing || last.Reason != "fix b" {
		t.Fatalf("got %+v", last)
	}
}

func TestBlockResume(t *testing.T) {
	active := []domain.Phase{
		domain.PhasePlanning, domain.PhaseAwaitingApproval, domain.PhaseWorking, domain.PhaseReviewing,
		domain.PhaseShipping, domain.PhaseReleasing, domain.PhaseObserving,
	}
	for _, phase := range active {
		t.Run(string(phase), func(t *testing.T) {
			blocked := apply(t, domain.Run{Phase: phase}, domain.Policy{}, domain.EventBlock, "stuck")
			if blocked.Phase != domain.PhaseBlocked || blocked.BlockedFrom != phase || blocked.Reason != "stuck" {
				t.Fatalf("got %+v", blocked)
			}
			resumed := apply(t, blocked, domain.Policy{}, domain.EventResume, "")
			if resumed.Phase != phase || resumed.BlockedFrom != domain.PhaseNone || resumed.Reason != "" {
				t.Fatalf("got %+v", resumed)
			}
		})
	}
}

func TestGoLivePhases(t *testing.T) {
	on := domain.Policy{GoLive: true}
	shipping := domain.Run{Phase: domain.PhaseShipping}

	releasing := apply(t, shipping, on, domain.EventComplete, "")
	observing := apply(t, releasing, on, domain.EventDeploy, "")
	done := apply(t, observing, on, domain.EventClose, "")
	got := []domain.Phase{releasing.Phase, observing.Phase, done.Phase}
	want := []domain.Phase{domain.PhaseReleasing, domain.PhaseObserving, domain.PhaseDone}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step %d: got %s want %s", i, got[i], want[i])
		}
	}

	if off := apply(t, shipping, domain.Policy{}, domain.EventComplete, ""); off.Phase != domain.PhaseDone {
		t.Fatalf("got %s", off.Phase)
	}
}

func TestGoLiveRounds(t *testing.T) {
	policy := domain.Policy{GoLive: true, MaxLiveRounds: 2}
	for _, phase := range []domain.Phase{domain.PhaseReleasing, domain.PhaseObserving} {
		t.Run(string(phase), func(t *testing.T) {
			run := domain.Run{Phase: phase, ReviewRound: 2, LiveRound: 0}
			replan := apply(t, run, policy, domain.EventRegress, "p99 breach")
			if replan.Phase != domain.PhasePlanning || replan.LiveRound != 1 || replan.ReviewRound != 0 || replan.Reason != "p99 breach" {
				t.Fatalf("got %+v", replan)
			}

			last := apply(t, domain.Run{Phase: phase, LiveRound: 1}, policy, domain.EventRegress, "again")
			if last.Phase != domain.PhaseBlocked || last.BlockedFrom != phase || last.Reason != "again" {
				t.Fatalf("got %+v", last)
			}
		})
	}
}

func TestCompleteRecordsPR(t *testing.T) {
	const first, second = "https://github.com/o/r/pull/1", "https://github.com/o/r/pull/2"
	run := domain.Run{Phase: domain.PhaseShipping, PRs: []string{first}}

	next, err := run.Apply(domain.Command{Event: domain.EventComplete, PRURL: second}, domain.Policy{GoLive: true})
	if err != nil {
		t.Fatal(err)
	}

	if next.PRURL != second || !reflect.DeepEqual(next.PRs, []string{first, second}) {
		t.Fatalf("got PRURL %q, PRs %v", next.PRURL, next.PRs)
	}
	if run.PRURL != "" || !reflect.DeepEqual(run.PRs, []string{first}) {
		t.Fatalf("old run changed: %+v", run)
	}
}

func TestRegressClearsIteration(t *testing.T) {
	const url = "https://github.com/o/r/pull/1"
	run := domain.Run{Phase: domain.PhaseObserving, PRURL: url, PRs: []string{url}}

	cases := []struct {
		name      string
		maxRounds int
		wantPhase domain.Phase
		wantPRURL string
	}{
		{name: "rounds left", maxRounds: 2, wantPhase: domain.PhasePlanning},
		{name: "last round keeps the PR for a resume", maxRounds: 1, wantPhase: domain.PhaseBlocked, wantPRURL: url},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := apply(t, run, domain.Policy{GoLive: true, MaxLiveRounds: tc.maxRounds}, domain.EventRegress, "p99 breach")

			if next.Phase != tc.wantPhase || next.PRURL != tc.wantPRURL || !reflect.DeepEqual(next.PRs, []string{url}) || next.LiveRound != 1 || next.Reason != "p99 breach" {
				t.Fatalf("got phase %s, PRURL %q, PRs %v, live round %d, reason %q", next.Phase, next.PRURL, next.PRs, next.LiveRound, next.Reason)
			}
		})
	}
}

func TestTransitionResetsStopBlocks(t *testing.T) {
	run := domain.Run{Phase: domain.PhasePlanning, StopBlocks: 2}
	if got := apply(t, run, domain.Policy{}, domain.EventAbandon, "").StopBlocks; got != 0 {
		t.Fatalf("got %d", got)
	}
}
