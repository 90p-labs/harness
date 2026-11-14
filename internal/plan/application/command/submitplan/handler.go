package submitplan

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	programRefusal = "harness run plan takes a single-run plan, not a program"
	mapRefusal     = "harness run plan takes a single-run plan, not a map"
)

var ErrPlanRejected = errors.New("plan rejected")

type Command struct {
	RunID     string
	Phase     string
	Path      string
	LiveRound int
}

type Result struct {
	Track          shareddomain.Track
	ApprovalWaived bool
	QuestionsOpen  bool
}

type payload struct {
	Path  string             `json:"path"`
	Track shareddomain.Track `json:"track"`
}

type Check func(ctx context.Context, path string, liveRound int) (domain.Report, error)

type Handler struct {
	check        Check
	bus          shareddomain.Dispatcher
	cwd          string
	reproduction domain.ReproductionReader
}

func NewHandler(check Check, bus shareddomain.Dispatcher, cwd string) Handler {
	return Handler{check: check, bus: bus, cwd: cwd}
}

// WithReproduction sets the reader of the red check. Without it the check does not run.
func (h Handler) WithReproduction(reader domain.ReproductionReader) Handler {
	h.reproduction = reader
	return h
}

func (h Handler) Handle(ctx context.Context, c Command) (Result, error) {
	report, err := h.check(ctx, c.Path, c.LiveRound)
	if err != nil {
		return Result{}, err
	}
	if report.Program != nil {
		kind := domain.Problem{Line: 1, Rule: domain.RulePlanKind, Message: programRefusal}
		return Result{}, fmt.Errorf("%w:\n%s", domain.ErrPlanHasProblems, kind)
	}
	if report.Map != nil {
		kind := domain.Problem{Line: 1, Rule: domain.RulePlanKind, Message: mapRefusal}
		return Result{}, fmt.Errorf("%w:\n%s", domain.ErrPlanHasProblems, kind)
	}
	problems := report.Problems
	if len(problems) == 0 && c.Phase == shareddomain.PhasePlanning {
		problems, err = domain.VerifyReproduction(ctx, report.Doc, h.reproduction)
		if err != nil {
			return Result{}, err
		}
	}
	if len(problems) > 0 {
		lines := make([]string, len(problems))
		for i, p := range problems {
			lines[i] = p.String()
		}
		return Result{}, fmt.Errorf("%w:\n%s", domain.ErrPlanHasProblems, strings.Join(lines, "\n"))
	}
	raw, err := json.Marshal(payload{Path: c.Path, Track: report.Track})
	if err != nil {
		return Result{}, err
	}
	result, err := h.bus.Dispatch(ctx, shareddomain.Envelope{
		Version: shareddomain.EnvelopeVersion,
		Event:   shareddomain.EventPlanSubmitted,
		Cwd:     h.cwd,
		Run:     shareddomain.RunRef{ID: c.RunID, Phase: c.Phase},
		Payload: raw,
	})
	if err != nil {
		return Result{}, fmt.Errorf("dispatch %s: %w", shareddomain.EventPlanSubmitted, err)
	}
	if reasons := rejection(result); len(reasons) > 0 {
		return Result{}, fmt.Errorf("%w: %s", ErrPlanRejected, strings.Join(reasons, "\n"))
	}
	return Result{Track: report.Track, ApprovalWaived: report.Doc.ApprovalWaived(), QuestionsOpen: report.Doc.QuestionsOpen()}, nil
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
