package page

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/90p-labs/harness/internal/panel/domain"
)

const regionsBase = "/runs/"

type Query struct {
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
	Events    string
}

type Handler struct {
	runs  domain.Runs
	pages domain.Pages
}

func NewHandler(runs domain.Runs, pages domain.Pages) Handler {
	return Handler{runs: runs, pages: pages}
}

func (h Handler) Handle(ctx context.Context, query Query) (Result, error) {
	card, err := h.runs.Get(ctx, query.RunID)
	if err != nil {
		return Result{}, err
	}
	current := card.Version
	served := domain.Served{
		Version: current.String(),
		Base:    regionsBase + card.ID + "/regions/",
		Events:  "/events?run=" + url.QueryEscape(card.ID),
	}
	if query.Region == "" {
		body, err := h.pages.Page(ctx, card, served)
		if errors.Is(err, domain.ErrNoPlan) {
			return Result{NoPlan: true, Card: card, Version: current.String(), Events: served.Events}, nil
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
