package domain

import "slices"

type TransitionFilter struct {
	Transition []string
	From       []string
	To         []string
}

func (f TransitionFilter) Empty() bool {
	return len(f.Transition) == 0 && len(f.From) == 0 && len(f.To) == 0
}

func (f TransitionFilter) Matches(p TransitionPayload) bool {
	return admits(f.Transition, p.Event) && admits(f.From, p.From) && admits(f.To, p.To)
}

func admits(allowed []string, value string) bool {
	return len(allowed) == 0 || slices.Contains(allowed, value)
}
