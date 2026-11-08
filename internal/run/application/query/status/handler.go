package status

import (
	"context"

	"github.com/jorge07/harness/internal/run/domain"
)

type Query struct {
	Key   domain.RunKey
	RunID string
}

type Status struct {
	ID          string
	Phase       domain.Phase
	ReviewRound int
	LiveRound   int
	Reason      string
	Prompt      string
}

type Handler struct {
	repo domain.RunRepository
}

func NewHandler(repo domain.RunRepository) Handler {
	return Handler{repo: repo}
}

func (h Handler) Handle(ctx context.Context, query Query) (Status, error) {
	var run domain.Run
	var err error
	if query.RunID != "" {
		run, err = h.repo.Get(ctx, query.RunID)
	} else {
		run, err = h.repo.Active(ctx, query.Key)
	}
	if err != nil {
		return Status{}, err
	}
	return Status{
		ID: run.ID, Phase: run.Phase, ReviewRound: run.ReviewRound,
		LiveRound: run.LiveRound, Reason: run.Reason, Prompt: run.Prompt,
	}, nil
}
