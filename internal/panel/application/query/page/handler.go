package page

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/90p-labs/harness/internal/panel/domain"
)

// runBase returns the route of a run of one repo, with no trailing slash.
func runBase(ref domain.RunRef) string {
	return "/repos/" + ref.Repo + "/runs/" + ref.ID
}

type Query struct {
	Repo    string
	RunID   string
	Region  string
	Version string
}

// Result is the answer of the page query. NoPlan marks a run with no plan.md:
// Card, Version and Events then hold what the no-plan page shows.
type Result struct {
	Body      []byte
	Unchanged bool
	Refresh   bool
	NoPlan    bool
	Card      domain.Card
	Version   string
	Base      string
	Events    string
}

type Handler struct {
	runs  domain.Runs
	pages domain.Pages
}

func NewHandler(runs domain.Runs, pages domain.Pages) Handler {
	return Handler{runs: runs, pages: pages}
}

// Resolve finds the repo of a run id for a legacy route. It returns the ref
// when exactly one repo holds the id, else ErrNotFound.
func (h Handler) Resolve(ctx context.Context, id string) (domain.RunRef, error) {
	listing, err := h.runs.List(ctx)
	if err != nil {
		return domain.RunRef{}, err
	}
	var found []domain.RunRef
	for _, card := range listing.Cards {
		if card.ID == id {
			found = append(found, card.Ref())
		}
	}
	// A repo that failed to load may hold the id too, so the match cannot be
	// known to be unique (ADR 0027 item 16).
	if len(listing.Problems) > 0 {
		return domain.RunRef{}, fmt.Errorf("run %q: %d repos failed to load: %w", id, len(listing.Problems), domain.ErrNotFound)
	}
	if len(found) != 1 {
		return domain.RunRef{}, fmt.Errorf("run %q in %d repos: %w", id, len(found), domain.ErrNotFound)
	}
	return found[0], nil
}

func (h Handler) Handle(ctx context.Context, query Query) (Result, error) {
	ref := domain.RunRef{Repo: query.Repo, ID: query.RunID}
	card, err := h.runs.Get(ctx, ref)
	if err != nil {
		return Result{}, err
	}
	current := card.Version
	served := domain.Served{
		Version: current.String(),
		Base:    runBase(ref) + "/regions/",
		Events:  "/events?repo=" + url.QueryEscape(ref.Repo) + "&run=" + url.QueryEscape(ref.ID),
	}
	if query.Region == "" {
		body, err := h.pages.Page(ctx, card, served)
		if errors.Is(err, domain.ErrNoPlan) {
			return Result{NoPlan: true, Card: card, Version: current.String(), Base: served.Base, Events: served.Events}, nil
		}
		if err != nil {
			return Result{}, err
		}
		return Result{Body: body}, nil
	}
	if !h.pages.HasRegion(query.Region) {
		return Result{}, fmt.Errorf("region %q: %w", query.Region, domain.ErrRegionNotFound)
	}
	sent := domain.ParseVersion(query.Version)
	if sent == current && sent != (domain.Version{}) {
		return Result{Unchanged: true}, nil
	}
	if sent.Plan != current.Plan {
		return Result{Refresh: true}, nil
	}
	body, err := h.pages.Region(ctx, card, query.Region, served)
	if errors.Is(err, domain.ErrNoPlan) {
		return Result{Unchanged: true}, nil
	}
	if err != nil {
		return Result{}, err
	}
	return Result{Body: body}, nil
}
