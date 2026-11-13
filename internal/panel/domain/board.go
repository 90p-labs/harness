package domain

import (
	"slices"
	"strings"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const doneLimit = 20

type Card struct {
	ID          string
	Title       string
	Phase       string
	BlockedFrom string
	Branch      string
	PRURL       string
	Worktree    string
	Version     Version
}

// Blocked reports whether the card is blocked.
func (c Card) Blocked() bool {
	return c.Phase == shareddomain.PhaseBlocked
}

type Column struct {
	Key    string
	Name   string
	Phases []string
	Cards  []Card
}

// NeedsYou reports whether the column is the approval column and holds a card.
func (c Column) NeedsYou() bool {
	return c.Key == "approval" && len(c.Cards) > 0
}

type Board struct {
	Columns []Column
	Version string
}

func columns() []Column {
	return []Column{
		{Key: "planning", Name: "Planning", Phases: []string{shareddomain.PhasePlanning}},
		{Key: "approval", Name: "Approval", Phases: []string{shareddomain.PhaseAwaitingApproval}},
		{Key: "working", Name: "Working", Phases: []string{shareddomain.PhaseWorking}},
		{Key: "reviewing", Name: "Reviewing", Phases: []string{shareddomain.PhaseReviewing}},
		{Key: "shipping", Name: "Shipping", Phases: []string{shareddomain.PhaseShipping, shareddomain.PhaseReleasing, shareddomain.PhaseObserving}},
		{Key: "done", Name: "Done", Phases: []string{shareddomain.PhaseDone, shareddomain.PhaseAbandoned}},
	}
}

// NewBoard groups the cards in six columns. A blocked card sits in the column
// of its BlockedFrom phase. Done keeps the 20 most recent cards by id.
func NewBoard(cards []Card) Board {
	cols := columns()
	for _, card := range cards {
		phase := card.Phase
		if card.Blocked() {
			phase = card.BlockedFrom
		}
		for i := range cols {
			if slices.Contains(cols[i].Phases, phase) {
				cols[i].Cards = append(cols[i].Cards, card)
				break
			}
		}
	}
	done := &cols[len(cols)-1]
	slices.SortStableFunc(done.Cards, func(a, b Card) int { return strings.Compare(b.ID, a.ID) })
	if len(done.Cards) > doneLimit {
		done.Cards = done.Cards[:doneLimit]
	}

	var parts []string
	for _, col := range cols {
		for _, card := range col.Cards {
			parts = append(parts, card.ID, card.Phase, card.Version.String())
		}
	}
	return Board{Columns: cols, Version: Fingerprint(parts...)}
}
