package board_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/panel/application/query/board"
	"github.com/90p-labs/harness/internal/panel/domain"
)

type runs struct {
	cards []domain.Card
	err   error
}

const repo = "aaaaaaaaaaaa"

func (r runs) List(context.Context) (domain.Listing, error) {
	return domain.Listing{Repos: []domain.Repo{{Key: repo, Name: "harness"}}, Cards: r.cards}, r.err
}
func (r runs) Get(context.Context, domain.RunRef) (domain.Card, error) {
	return domain.Card{}, domain.ErrNotFound
}

func TestListErrorReturnsZeroResult(t *testing.T) {
	boom := errors.New("list failed")
	h := board.NewHandler(runs{cards: []domain.Card{{ID: "r1", Phase: "working"}}, err: boom})
	res, err := h.Handle(context.Background(), board.Query{})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if !reflect.DeepEqual(res, board.Result{}) {
		t.Errorf("result = %+v, want zero Result", res)
	}
}

func TestUnknownRepoBeatsUnchanged(t *testing.T) {
	source := runs{cards: []domain.Card{{Repo: repo, ID: "r1", Phase: "working"}}}
	h := board.NewHandler(source)
	listing, _ := source.List(context.Background())
	const missing = "eeeeeeeeeeee"
	v := domain.NewBoard(listing, missing).Version
	res, err := h.Handle(context.Background(), board.Query{Repo: missing, Version: v})
	if err != nil || !res.UnknownRepo || res.Unchanged || len(res.Board.Columns) != 0 {
		t.Fatalf("res = %+v, %v, want UnknownRepo only", res, err)
	}
	known, err := h.Handle(context.Background(), board.Query{Repo: repo})
	if err != nil || known.UnknownRepo || known.Board.Filter != repo {
		t.Fatalf("known = %+v, %v", known, err)
	}
}

func TestUnchangedVersion(t *testing.T) {
	h := board.NewHandler(runs{cards: []domain.Card{{ID: "r1", Phase: "working", Version: domain.Version{Plan: "p", Run: "r"}}}})
	first, err := h.Handle(context.Background(), board.Query{})
	if err != nil || first.Unchanged || first.Board.Version == "" {
		t.Fatalf("first = %+v, %v", first, err)
	}
	again, err := h.Handle(context.Background(), board.Query{Version: first.Board.Version})
	if err != nil || !again.Unchanged {
		t.Fatalf("again = %+v, %v", again, err)
	}
	stale, _ := h.Handle(context.Background(), board.Query{Version: "stale"})
	if stale.Unchanged {
		t.Fatal("stale version reported unchanged")
	}
}
