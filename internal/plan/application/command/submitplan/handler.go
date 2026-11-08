package submitplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jorge07/harness/internal/plan/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

var ErrPlanRejected = errors.New("plan rejected")

type Command struct {
	RunID     string
	Phase     string
	Path      string
	LiveRound int
}

type payload struct {
	Path  string             `json:"path"`
	Track shareddomain.Track `json:"track"`
}

type Check func(ctx context.Context, path string, liveRound int) (domain.Report, error)

type Handler struct {
	check Check
	bus   shareddomain.Dispatcher
	cwd   string
}

func NewHandler(check Check, bus shareddomain.Dispatcher, cwd string) Handler {
	return Handler{check: check, bus: bus, cwd: cwd}
}

func (h Handler) Handle(ctx context.Context, c Command) (shareddomain.Track, error) {
	report, err := h.check(ctx, c.Path, c.LiveRound)
	if err != nil {
		return "", err
	}
	if len(report.Problems) > 0 {
		lines := make([]string, len(report.Problems))
		for i, p := range report.Problems {
			lines[i] = p.String()
		}
		return "", fmt.Errorf("%w:\n%s", domain.ErrPlanHasProblems, strings.Join(lines, "\n"))
	}
	raw, err := json.Marshal(payload{Path: c.Path, Track: report.Track})
	if err != nil {
		return "", err
	}
	result, err := h.bus.Dispatch(ctx, shareddomain.Envelope{
		Version: shareddomain.EnvelopeVersion,
		Event:   shareddomain.EventPlanSubmitted,
		Cwd:     h.cwd,
		Run:     shareddomain.RunRef{ID: c.RunID, Phase: c.Phase},
		Payload: raw,
	})
	if err != nil {
		return "", fmt.Errorf("dispatch %s: %w", shareddomain.EventPlanSubmitted, err)
	}
	if reasons := rejection(result); len(reasons) > 0 {
		return "", fmt.Errorf("%w: %s", ErrPlanRejected, strings.Join(reasons, "\n"))
	}
	return report.Track, nil
}

func rejection(result shareddomain.DispatchResult) []string {
	var reasons []string
	if result.Verdict.Kind == shareddomain.VerdictDeny {
		reasons = append(reasons, result.Verdict.Reason)
	}
	for _, f := range result.Failures {
		reasons = append(reasons, fmt.Sprintf("%s: %v", f.Plugin, f.Err))
	}
	return reasons
}
