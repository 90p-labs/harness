package start

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/90p-labs/harness/internal/run/application/command/transition"
	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	idTimeLayout = "20060102-150405"
	slugMaxLen   = 40
	slugDash     = "-"
)

var nonSlugRuns = regexp.MustCompile(`[^a-z0-9]+`)

type Command struct {
	Key    domain.RunKey
	Prompt string
}

type Handler struct {
	repo  domain.RunRepository
	flow  transition.Flow
	clock func() time.Time
}

func NewHandler(repo domain.RunRepository, flow transition.Flow, clock func() time.Time) Handler {
	return Handler{repo: repo, flow: flow, clock: clock}
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	_, err := h.repo.Active(ctx, cmd.Key)
	if err == nil {
		return domain.ErrActiveRunExists
	}
	if !errors.Is(err, domain.ErrRunNotFound) {
		return err
	}
	run := domain.Run{ID: newRunID(h.clock(), cmd.Prompt), Key: cmd.Key, Prompt: cmd.Prompt}
	return h.flow.Move(ctx, run, domain.Command{Event: domain.EventStart})
}

func newRunID(at time.Time, prompt string) string {
	slug := nonSlugRuns.ReplaceAllString(strings.ToLower(prompt), slugDash)
	slug = strings.Trim(slug, slugDash)
	if len(slug) > slugMaxLen {
		slug = strings.TrimRight(slug[:slugMaxLen], slugDash)
	}
	id := at.Format(idTimeLayout)
	if slug == "" {
		return id
	}
	return id + slugDash + slug
}
