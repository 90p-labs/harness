package status

import (
	"context"

	"github.com/90p-labs/harness/internal/run/domain"
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
	Worktree    string
	PRURL       string
	Version     int
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
	return toStatus(run), nil
}

func (h Handler) ByPhase(ctx context.Context, repo string, phases []domain.Phase) ([]Status, error) {
	runs, err := h.repo.ListByPhase(ctx, repo, phases)
	if err != nil {
		return nil, err
	}
	statuses := make([]Status, len(runs))
	for i, run := range runs {
		statuses[i] = toStatus(run)
	}
	return statuses, nil
}

func toStatus(run domain.Run) Status {
	return Status{
		ID: run.ID, Phase: run.Phase, ReviewRound: run.ReviewRound,
		LiveRound: run.LiveRound, Reason: run.Reason, Prompt: run.Prompt,
		Worktree: run.Key.Worktree, PRURL: run.PRURL, Version: run.Version,
	}
}
