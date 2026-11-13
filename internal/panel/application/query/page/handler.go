package page

import (
	"context"
	"fmt"

	"github.com/90p-labs/harness/internal/panel/domain"
)

const regionsBase = "/runs/"

type Query struct {
	RunID   string
	Region  string
	Version string
}

type Result struct {
	Body      []byte
	Unchanged bool
	Refresh   bool
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
	}
	if query.Region == "" {
		body, err := h.pages.Page(ctx, card, served)
		if err != nil {
			return Result{}, err
		}
		return Result{Body: body}, nil
	}
	if !h.pages.HasRegion(query.Region) {
		return Result{}, fmt.Errorf("region %q: %w", query.Region, domain.ErrNotFound)
	}
	sent := domain.ParseVersion(query.Version)
	if sent == current && sent != (domain.Version{}) {
		return Result{Unchanged: true}, nil
	}
	if sent.Plan != current.Plan {
		return Result{Refresh: true}, nil
	}
	body, err := h.pages.Region(ctx, card, query.Region, served)
	if err != nil {
		return Result{}, err
	}
	return Result{Body: body}, nil
}
