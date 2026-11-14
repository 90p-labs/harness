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
	// Repo is the key of the repo that holds the run.
	Repo string
	// RepoName is the name of the repo, filled by NewBoard from the listing.
	RepoName    string
	ID          string
	Title       string
	Phase       string
	BlockedFrom string
	Branch      string
	PRURL       string
	Worktree    string
	Version     Version
}

// Ref names the run of the card.
func (c Card) Ref() RunRef { return RunRef{Repo: c.Repo, ID: c.ID} }

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

// RepoItem is one entry of the repo list. Count is the number of cards the
// full board shows for the repo, with Done capped. NeedsYou marks a repo with
// a card in the approval column. Both ignore the filter.
type RepoItem struct {
	Key      string
	Name     string
	Path     string
	Count    int
	NeedsYou bool
}

// Problem is the notice of one repo that failed to load.
type Problem struct {
	Repo string
	Name string
	Text string
}

// Line returns the notice as "<repo name>: <problem>".
func (p Problem) Line() string { return p.Name + ": " + p.Text }

// Board is one set of six columns that holds the cards of every repo, or of
// the filtered repo. The repo list, the total and the marks ignore the filter.
type Board struct {
	// Filter is the key of the repo the columns show, or "" for every repo.
	Filter   string
	Repos    []RepoItem
	Total    int
	NeedsYou bool
	Problems []Problem
	Columns  []Column
	Version  string
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

// columnIndex returns the index of the column that holds the card, or -1. A
// blocked card sits in the column of its BlockedFrom phase.
func columnIndex(cols []Column, card Card) int {
	phase := card.Phase
	if card.Blocked() {
		phase = card.BlockedFrom
	}
	return slices.IndexFunc(cols, func(c Column) bool { return slices.Contains(c.Phases, phase) })
}

// newestFirst orders cards by run id, newest first, then by repo key.
func newestFirst(a, b Card) int {
	if c := strings.Compare(b.ID, a.ID); c != 0 {
		return c
	}
	return strings.Compare(a.Repo, b.Repo)
}

// NewBoard builds the board of the listing. An empty filter shows the cards of
// every repo, and a repo key shows only the cards and the problem of that repo.
// A card of a repo that is not in the listing shows nowhere. Done keeps the 20
// most recent cards of each repo, so the full board is the union of the
// filtered boards. The repo list sorts by the newest run, and a repo with no
// run sorts last, in listing order. Its counts and marks ignore the filter.
// The version hashes the filter, the repo list, the problems and the cards,
// with the repo key of each card.
func NewBoard(listing Listing, filter string) Board {
	names := make(map[string]string, len(listing.Repos))
	for _, repo := range listing.Repos {
		names[repo.Key] = repo.Name
	}
	byRepo := map[string][]Card{}
	for _, card := range listing.Cards {
		if name, ok := names[card.Repo]; ok {
			card.RepoName = name
			byRepo[card.Repo] = append(byRepo[card.Repo], card)
		}
	}

	type ranked struct {
		item   RepoItem
		newest string
	}
	board := Board{Filter: filter, Columns: columns()}
	done := slices.IndexFunc(board.Columns, func(c Column) bool { return c.Key == "done" })
	all := make([]ranked, 0, len(listing.Repos))
	for _, repo := range listing.Repos {
		cards := byRepo[repo.Key]
		slices.SortStableFunc(cards, newestFirst)
		r := ranked{item: RepoItem{Key: repo.Key, Name: repo.Name, Path: repo.Worktree}}
		kept := 0
		for _, card := range cards {
			r.newest = max(r.newest, card.ID)
			i := columnIndex(board.Columns, card)
			if i < 0 || (i == done && kept >= doneLimit) {
				continue
			}
			if i == done {
				kept++
			}
			r.item.Count++
			if board.Columns[i].Key == "approval" {
				r.item.NeedsYou = true
			}
			if filter == "" || filter == repo.Key {
				board.Columns[i].Cards = append(board.Columns[i].Cards, card)
			}
		}
		all = append(all, r)
	}
	for i := range board.Columns {
		slices.SortStableFunc(board.Columns[i].Cards, newestFirst)
	}
	slices.SortStableFunc(all, func(a, b ranked) int { return strings.Compare(b.newest, a.newest) })

	var parts []string
	parts = append(parts, "filter", filter)
	for _, r := range all {
		board.Repos = append(board.Repos, r.item)
		board.Total += r.item.Count
		board.NeedsYou = board.NeedsYou || r.item.NeedsYou
		parts = append(parts, "repo", r.item.Key, r.item.Name, r.item.Path, fmt.Sprint(r.item.Count), fmt.Sprint(r.item.NeedsYou))
		if text := listing.Problems[r.item.Key]; text != "" && (filter == "" || filter == r.item.Key) {
			board.Problems = append(board.Problems, Problem{Repo: r.item.Key, Name: r.item.Name, Text: text})
			parts = append(parts, "problem", r.item.Key, text)
		}
	}
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			parts = append(parts, "card", card.Repo, card.ID, card.Phase, card.Version.String())
		}
	}
	board.Version = Fingerprint(parts...)
	return board
}
