package page_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/domain"
)

const repo = "aaaaaaaaaaaa"

var card = domain.Card{Repo: repo, ID: "r1", Phase: "working", Version: domain.Version{Plan: "p1", Run: "r1"}}

type runs struct{ extra []domain.Card }

func (r runs) List(context.Context) (domain.Listing, error) {
	return domain.Listing{Cards: append([]domain.Card{card}, r.extra...)}, nil
}
func (runs) Get(_ context.Context, ref domain.RunRef) (domain.Card, error) {
	if ref != card.Ref() {
		return domain.Card{}, domain.ErrNotFound
	}
	return card, nil
}

func TestResolve(t *testing.T) {
	h := page.NewHandler(runs{extra: []domain.Card{
		{Repo: "bbbbbbbbbbbb", ID: "r2"}, {Repo: repo, ID: "twice"}, {Repo: "bbbbbbbbbbbb", ID: "twice"},
	}}, &pages{})
	if ref, err := h.Resolve(context.Background(), "r2"); err != nil || ref != (domain.RunRef{Repo: "bbbbbbbbbbbb", ID: "r2"}) {
		t.Errorf("unique id = %+v, %v", ref, err)
	}
	for _, id := range []string{"twice", "gone"} {
		if _, err := h.Resolve(context.Background(), id); !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", id, err)
		}
	}
}

// failedRuns lists a repo that failed to load next to the repo that holds r1.
type failedRuns struct{ runs }

func (r failedRuns) List(ctx context.Context) (domain.Listing, error) {
	listing, _ := r.runs.List(ctx)
	listing.Repos = []domain.Repo{{Key: repo}, {Key: "cccccccccccc"}}
	listing.Problems = map[string]string{"cccccccccccc": "state unreadable"}
	return listing, nil
}

func TestResolveFailedRepoIsNotUnique(t *testing.T) {
	h := page.NewHandler(failedRuns{}, &pages{})
	if ref, err := h.Resolve(context.Background(), "r1"); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("id with a failed repo = %+v, %v, want ErrNotFound", ref, err)
	}
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
	res, err := h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1", Region: "criteria", Version: card.Version.String()})
	if err != nil || !res.Unchanged || ps.regionCalls != 0 {
		t.Fatalf("res = %+v, calls = %d, err = %v", res, ps.regionCalls, err)
	}
	res, err = h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1", Region: "criteria", Version: "p1.other"})
	if err != nil || res.Unchanged || res.Refresh || string(res.Body) != "region" {
		t.Fatalf("stale run part = %+v, %v", res, err)
	}
	res, err = h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1", Version: card.Version.String()})
	if err != nil || res.Unchanged || string(res.Body) != "page" {
		t.Fatalf("full page must render: %+v, %v", res, err)
	}
	if _, err := h.Handle(context.Background(), page.Query{Repo: repo, RunID: "nope"}); err == nil {
		t.Fatal("unknown run must fail")
	}
}

func TestUnknownRegionNotFound(t *testing.T) {
	ps := &pages{}
	h := page.NewHandler(runs{}, ps)
	for _, v := range []string{"", card.Version.String(), "old.r1", "p1.other"} {
		_, err := h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1", Region: "nope", Version: v})
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
		{Repo: repo, RunID: "r1"},
		{Repo: repo, RunID: "r1", Region: "criteria", Version: "p1.other"},
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
	res, err := h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1", Region: "specs", Version: "old.r1"})
	if err != nil || !res.Refresh || res.Unchanged || len(res.Body) != 0 || ps.regionCalls != 0 {
		t.Fatalf("res = %+v, calls = %d, err = %v", res, ps.regionCalls, err)
	}
	res, _ = h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1", Region: "specs", Version: "garbage"})
	if !res.Refresh {
		t.Fatalf("malformed version must refresh: %+v", res)
	}
}

type noPlanPages struct{ pages }

func (*noPlanPages) Page(context.Context, domain.Card, domain.Served) ([]byte, error) {
	return nil, domain.ErrNoPlan
}

func (*noPlanPages) Region(context.Context, domain.Card, string, domain.Served) ([]byte, error) {
	return nil, domain.ErrNoPlan
}

type servedPages struct {
	pages
	served domain.Served
}

func (p *servedPages) Page(_ context.Context, _ domain.Card, s domain.Served) ([]byte, error) {
	p.served = s
	return []byte("page"), nil
}

func TestNoPlan(t *testing.T) {
	h := page.NewHandler(runs{}, &noPlanPages{})
	res, err := h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1"})
	if err != nil || !res.NoPlan || res.Card.ID != "r1" || res.Version != card.Version.String() || len(res.Body) != 0 {
		t.Fatalf("page res = %+v, err = %v", res, err)
	}
	if res.Events != "/events?repo="+repo+"&run=r1" || res.Base != "/repos/"+repo+"/runs/r1/regions/" {
		t.Fatalf("no-plan Events = %q, Base = %q", res.Events, res.Base)
	}
	res, err = h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1", Region: "summary", Version: "p1.other"})
	if err != nil || !res.Unchanged || res.Refresh {
		t.Fatalf("region res = %+v, err = %v", res, err)
	}
	res, err = h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1", Region: "summary", Version: "old.r1"})
	if err != nil || !res.Refresh {
		t.Fatalf("plan appeared must refresh: %+v, %v", res, err)
	}
}

func TestServedEvents(t *testing.T) {
	ps := &servedPages{}
	h := page.NewHandler(runs{}, ps)
	if _, err := h.Handle(context.Background(), page.Query{Repo: repo, RunID: "r1"}); err != nil {
		t.Fatal(err)
	}
	if ps.served.Events != "/events?repo="+repo+"&run=r1" {
		t.Fatalf("Events = %q", ps.served.Events)
	}
}
