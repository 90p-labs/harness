package domain

import (
	"fmt"
	"slices"
	"strings"
	"time"

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

// PRNumber returns the trailing number of the PR url, or "" when it has none.
func (c Card) PRNumber() string {
	url := strings.TrimRight(c.PRURL, "/")
	last := url[strings.LastIndex(url, "/")+1:]
	if last == "" || strings.Trim(last, "0123456789") != "" {
		return ""
	}
	return last
}

const (
	shortTitleLimit = 80
	idStampLayout   = "20060102-150405"
	idStampLen      = len(idStampLayout)
)

// ShortTitle returns the first line of the title, cut to 80 runes. A longer
// line keeps 79 runes and ends with an ellipsis.
func (c Card) ShortTitle() string {
	line, _, _ := strings.Cut(strings.TrimSpace(c.Title), "\n")
	line = strings.TrimSpace(line)
	runes := []rune(line)
	if len(runes) <= shortTitleLimit {
		return line
	}
	return string(runes[:shortTitleLimit-1]) + "…"
}

// stamp parses the timestamp that starts the run id, as YYYYMMDD-HHMMSS. The
// run id is written in the local time zone, so the parse uses time.Local.
func (c Card) stamp() (time.Time, bool) {
	if len(c.ID) < idStampLen || (len(c.ID) > idStampLen && c.ID[idStampLen] != '-') {
		return time.Time{}, false
	}
	t, err := time.ParseInLocation(idStampLayout, c.ID[:idStampLen], time.Local)
	return t, err == nil
}

// ShortID returns the timestamp part of the run id, or "" when it has none.
func (c Card) ShortID() string {
	if _, ok := c.stamp(); !ok {
		return ""
	}
	return c.ID[:idStampLen]
}

// Age returns the time from the run id timestamp to now as "12m", "3h" or "2d".
// It parses the timestamp in the local time zone, so now should come from the
// same zone. It returns "" when the id has no timestamp, and "0m" for a future one.
func (c Card) Age(now time.Time) string {
	t, ok := c.stamp()
	if !ok {
		return ""
	}
	d := max(now.Sub(t), 0)
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	default:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	}
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
