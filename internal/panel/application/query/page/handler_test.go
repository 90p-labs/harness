package page_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/domain"
)

var card = domain.Card{ID: "r1", Phase: "working", Version: domain.Version{Plan: "p1", Run: "r1"}}

type runs struct{}

func (runs) List(context.Context) ([]domain.Card, error) { return []domain.Card{card}, nil }
func (runs) Get(_ context.Context, id string) (domain.Card, error) {
	if id != card.ID {
		return domain.Card{}, domain.ErrNotFound
	}
	return card, nil
}

type pages struct{ pageCalls, regionCalls int }

func (p *pages) Page(context.Context, domain.Card, domain.Served) ([]byte, error) {
	p.pageCalls++
	return []byte("page"), nil
}

func (p *pages) HasRegion(name string) bool { return name != "nope" }

func (p *pages) Region(context.Context, domain.Card, string, domain.Served) ([]byte, error) {
	p.regionCalls++
	return []byte("region"), nil
}

func TestUnchangedVersion(t *testing.T) {
	ps := &pages{}
	h := page.NewHandler(runs{}, ps)
	res, err := h.Handle(context.Background(), page.Query{RunID: "r1", Region: "criteria", Version: card.Version.String()})
	if err != nil || !res.Unchanged || ps.regionCalls != 0 {
		t.Fatalf("res = %+v, calls = %d, err = %v", res, ps.regionCalls, err)
	}
	res, err = h.Handle(context.Background(), page.Query{RunID: "r1", Region: "criteria", Version: "p1.other"})
	if err != nil || res.Unchanged || res.Refresh || string(res.Body) != "region" {
		t.Fatalf("stale run part = %+v, %v", res, err)
	}
	res, err = h.Handle(context.Background(), page.Query{RunID: "r1", Version: card.Version.String()})
	if err != nil || res.Unchanged || string(res.Body) != "page" {
		t.Fatalf("full page must render: %+v, %v", res, err)
	}
	if _, err := h.Handle(context.Background(), page.Query{RunID: "nope"}); err == nil {
		t.Fatal("unknown run must fail")
	}
}

func TestUnknownRegionNotFound(t *testing.T) {
	ps := &pages{}
	h := page.NewHandler(runs{}, ps)
	for _, v := range []string{"", card.Version.String(), "old.r1", "p1.other"} {
		_, err := h.Handle(context.Background(), page.Query{RunID: "r1", Region: "nope", Version: v})
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("version %q: err = %v, want ErrNotFound", v, err)
		}
	}
	if ps.regionCalls != 0 {
		t.Errorf("unknown region rendered %d times", ps.regionCalls)
	}
}

type failingPages struct{}

func (failingPages) Page(context.Context, domain.Card, domain.Served) ([]byte, error) {
	return []byte("partial"), errors.New("render failed")
}

func (failingPages) HasRegion(string) bool { return true }

func (failingPages) Region(context.Context, domain.Card, string, domain.Served) ([]byte, error) {
	return []byte("partial"), errors.New("render failed")
}

func TestRenderErrorReturnsZeroResult(t *testing.T) {
	h := page.NewHandler(runs{}, failingPages{})
	for _, q := range []page.Query{
		{RunID: "r1"},
		{RunID: "r1", Region: "criteria", Version: "p1.other"},
	} {
		res, err := h.Handle(context.Background(), q)
		if err == nil {
			t.Fatalf("query %+v: want an error", q)
		}
		if !reflect.DeepEqual(res, page.Result{}) {
			t.Errorf("query %+v: result = %+v, want zero Result", q, res)
		}
	}
}

func TestPlanChangeRefreshes(t *testing.T) {
	ps := &pages{}
	h := page.NewHandler(runs{}, ps)
	res, err := h.Handle(context.Background(), page.Query{RunID: "r1", Region: "specs", Version: "old.r1"})
	if err != nil || !res.Refresh || res.Unchanged || len(res.Body) != 0 || ps.regionCalls != 0 {
		t.Fatalf("res = %+v, calls = %d, err = %v", res, ps.regionCalls, err)
	}
	res, _ = h.Handle(context.Background(), page.Query{RunID: "r1", Region: "specs", Version: "garbage"})
	if !res.Refresh {
		t.Fatalf("malformed version must refresh: %+v", res)
	}
}
