package status_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jorge07/harness/internal/run/application/query/status"
	"github.com/jorge07/harness/internal/run/domain"
)

var key = domain.RunKey{Repo: "github.com/jorge07/harness", Worktree: "/work/harness"}

type fakeRepository struct {
	open   *domain.Run
	byID   map[string]domain.Run
	getIDs []string
}

func (f *fakeRepository) Active(_ context.Context, k domain.RunKey) (domain.Run, error) {
	if f.open == nil || f.open.Key != k {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return *f.open, nil
}

func (f *fakeRepository) Get(_ context.Context, id string) (domain.Run, error) {
	f.getIDs = append(f.getIDs, id)
	run, ok := f.byID[id]
	if !ok {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return run, nil
}

func (f *fakeRepository) Save(context.Context, domain.Run) (domain.Run, error) {
	return domain.Run{}, errors.New("a query never saves")
}

func TestStatusQuery(t *testing.T) {
	open := domain.Run{
		ID: "20261108-011805-add-login", Key: key, Prompt: "Add login", Phase: domain.PhaseWorking,
		ReviewRound: 1, LiveRound: 2, Reason: "the tests miss the empty case", Version: 7,
	}
	closed := domain.Run{ID: "20261101-090000-old", Key: key, Prompt: "Old", Phase: domain.PhaseDone}
	repo := &fakeRepository{open: &open, byID: map[string]domain.Run{open.ID: open, closed.ID: closed}}
	handler := status.NewHandler(repo)

	t.Run("by key reads the open run", func(t *testing.T) {
		got, err := handler.Handle(context.Background(), status.Query{Key: key})
		want := status.Status{
			ID: open.ID, Phase: domain.PhaseWorking, ReviewRound: 1, LiveRound: 2,
			Reason: "the tests miss the empty case", Prompt: "Add login",
		}
		if err != nil || got != want {
			t.Fatalf("got %+v, %v, want %+v", got, err, want)
		}
	})

	t.Run("by run id reads a closed run", func(t *testing.T) {
		got, err := handler.Handle(context.Background(), status.Query{Key: key, RunID: closed.ID})
		if err != nil || got.ID != closed.ID || got.Phase != domain.PhaseDone {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("no open run", func(t *testing.T) {
		_, err := status.NewHandler(&fakeRepository{}).Handle(context.Background(), status.Query{Key: key})
		if !errors.Is(err, domain.ErrRunNotFound) {
			t.Fatalf("err = %v", err)
		}
	})
}
