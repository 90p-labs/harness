package board

import (
	"context"

	"github.com/90p-labs/harness/internal/panel/domain"
)

type Query struct {
	Version string
}

type Result struct {
	Board     domain.Board
	Unchanged bool
}

type Handler struct {
	runs domain.Runs
}

func NewHandler(runs domain.Runs) Handler {
	return Handler{runs: runs}
}

func (h Handler) Handle(ctx context.Context, query Query) (Result, error) {
	cards, err := h.runs.List(ctx)
	if err != nil {
		return Result{}, err
	}
	board := domain.NewBoard(cards)
	if query.Version != "" && query.Version == board.Version {
		return Result{Unchanged: true}, nil
	}
	return Result{Board: board}, nil
}
