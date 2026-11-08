package domain

import "fmt"

type Run struct {
	ID          string
	Phase       Phase
	ReviewRound int
	LiveRound   int
	StopBlocks  int
	BlockedFrom Phase
	Reason      string
}

func (r Run) Apply(cmd Command, policy Policy) (Run, error) {
	row, ok := r.find(cmd.Event, policy)
	if !ok {
		return Run{}, fmt.Errorf("%w: event %q in phase %q", ErrIllegalTransition, cmd.Event, r.Phase)
	}
	next := r
	next.Phase = row.To
	next.StopBlocks = 0

	switch {
	case row.To == PhaseBlocked:
		next.BlockedFrom = r.Phase
		next.Reason = cmd.Reason
	case cmd.Event == EventResume:
		next.BlockedFrom = PhaseNone
		next.Reason = ""
	case cmd.Event == EventApprovePlan:
		next.ReviewRound = 0
	case cmd.Event == EventRequestRework:
		next.ReviewRound++
		next.Reason = cmd.Reason
	case cmd.Event == EventRegress:
		next.LiveRound++
		next.ReviewRound = 0
		next.Reason = cmd.Reason
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
