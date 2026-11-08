package domain

import (
	"fmt"
	"slices"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type Run struct {
	ID          string
	Key         RunKey
	Prompt      string
	Track       shareddomain.Track
	Phase       Phase
	ReviewRound int
	LiveRound   int
	StopBlocks  int
	BlockedFrom Phase
	Reason      string
	PRURL       string
	PRs         []string
	Version     int
	events      []Transitioned
}

func (r Run) Events() []Transitioned { return slices.Clone(r.events) }

func (r Run) Persisted() Run {
	r.Version++
	r.events = nil
	return r
}

func (r Run) Apply(cmd Command, policy Policy) (Run, error) {
	row, ok := r.find(cmd.Event, policy)
	if !ok {
		return Run{}, fmt.Errorf("%w: event %q in phase %q", ErrIllegalTransition, cmd.Event, r.Phase)
	}
	next := r
	next.Phase = row.To
	next.StopBlocks = 0
	next.events = append(slices.Clone(r.events), Transitioned{
		RunID: r.ID, From: r.Phase, Event: cmd.Event, To: row.To, Reason: cmd.Reason, At: cmd.At,
	})

	switch {
	case row.To == PhaseBlocked:
		next.BlockedFrom = r.Phase
		next.Reason = cmd.Reason
	case cmd.Event == EventResume:
		next.BlockedFrom = PhaseNone
		next.Reason = ""
	case cmd.Event == EventSubmitPlan || cmd.Event == EventAmend:
		next.Track = shareddomain.Stricter(r.Track, cmd.Track)
	case cmd.Event == EventApprovePlan:
		next.Track = shareddomain.Stricter(r.Track, cmd.Track)
		next.ReviewRound = 0
	case cmd.Event == EventRequestRework:
		next.ReviewRound++
		next.Reason = cmd.Reason
	case cmd.Event == EventComplete:
		next.PRURL = cmd.PRURL
		next.PRs = append(slices.Clone(r.PRs), cmd.PRURL)
	}
	if cmd.Event == EventRegress {
		next.LiveRound++
		next.ReviewRound = 0
		next.Reason = cmd.Reason
	}
	if cmd.Event == EventRegress && row.To == PhasePlanning {
		next.PRURL = ""
	}
	return next, nil
}

func (r Run) find(event Event, policy Policy) (Transition, bool) {
	for _, row := range table {
		if row.From == r.Phase && row.Event == event && row.Guard.Holds(r, policy) {
			return row, true
		}
	}
	return Transition{}, false
}
