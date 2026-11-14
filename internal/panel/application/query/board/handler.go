package board

import (
	"context"

	"github.com/90p-labs/harness/internal/panel/domain"
)

// Query asks for the board. Repo is the key of the repo filter, or "" for
// every repo. The caller parses the key before it asks.
type Query struct {
	Repo    string
	Version string
}

// Result holds the board. UnknownRepo reports a Repo that the listing does not
// hold, and then Board is empty and Unchanged is false.
type Result struct {
	Board       domain.Board
	Unchanged   bool
	UnknownRepo bool
}

type Handler struct {
	runs domain.Runs
}

func NewHandler(runs domain.Runs) Handler {
	return Handler{runs: runs}
}

func (h Handler) Handle(ctx context.Context, query Query) (Result, error) {
	listing, err := h.runs.List(ctx)
	if err != nil {
		return Result{}, err
	}
	// An unknown repo answers before the version compare, so a matching
	// version never hides it.
	if query.Repo != "" && !hasRepo(listing, query.Repo) {
		return Result{UnknownRepo: true}, nil
	}
	board := domain.NewBoard(listing, query.Repo)
	if query.Version != "" && query.Version == board.Version {
		return Result{Unchanged: true}, nil
	}
	return Result{Board: board}, nil
}

func hasRepo(listing domain.Listing, key string) bool {
	for _, repo := range listing.Repos {
		if repo.Key == key {
			return true
		}
	}
	return false
}
