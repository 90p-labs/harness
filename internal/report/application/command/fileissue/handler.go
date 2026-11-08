package fileissue

import (
	"context"

	"github.com/jorge07/harness/internal/report/domain"
)

type Command struct {
	Draft       domain.Draft
	Environment domain.Environment
	DryRun      bool
}

type Result struct {
	Issue domain.Draft
	URL   string
}

type Handler struct {
	tracker domain.Tracker
}

func NewHandler(tracker domain.Tracker) Handler {
	return Handler{tracker: tracker}
}

func (h Handler) Handle(ctx context.Context, c Command) (Result, error) {
	if err := c.Draft.Check(); err != nil {
		return Result{}, err
	}
	issue := c.Draft.Issue(c.Environment)
	if c.DryRun {
		return Result{Issue: issue}, nil
	}
	url, err := h.tracker.Create(ctx, issue.Title, issue.Body)
	if err != nil {
		return Result{}, err
	}
	return Result{Issue: issue, URL: url}, nil
}
