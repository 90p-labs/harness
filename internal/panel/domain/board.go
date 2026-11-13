package domain

import (
	"slices"
	"strings"
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

type Column struct {
	Name   string
	Phases []string
	Cards  []Card
}

type Board struct {
	Columns []Column
	Version string
}

func columns() []Column {
	return []Column{
		{Name: "Planning", Phases: []string{"planning"}},
		{Name: "Approval", Phases: []string{"awaiting_approval"}},
		{Name: "Working", Phases: []string{"working"}},
		{Name: "Reviewing", Phases: []string{"reviewing"}},
		{Name: "Shipping", Phases: []string{"shipping", "releasing", "observing"}},
		{Name: "Done", Phases: []string{"done", "abandoned"}},
	}
}

// NewBoard groups the cards in six columns. A blocked card sits in the column
// of its BlockedFrom phase. Done keeps the 20 most recent cards by id.
func NewBoard(cards []Card) Board {
	cols := columns()
	for _, card := range cards {
		phase := card.Phase
		if phase == "blocked" {
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
