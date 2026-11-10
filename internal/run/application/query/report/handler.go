package report

import (
	"context"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	spanReport = "run.report"
	attrRunID  = attribute.Key("harness.run.id")
)

type Query struct {
	Key   domain.RunKey
	RunID string
}

type PhaseMinutes struct {
	Phase   domain.Phase
	Minutes float64
}

type Report struct {
	ID           string
	Phase        domain.Phase
	ReworkRounds int
	Amends       int
	StopBlocks   int
	Classes      []domain.ClassCount
	Minutes      []PhaseMinutes
	TraceID      string
}

type Handler struct {
	repo    domain.RunRepository
	log     domain.TransitionLog
	now     func() time.Time
	traceID func(runID string) string
	tracer  trace.Tracer
}

func NewHandler(repo domain.RunRepository, log domain.TransitionLog, now func() time.Time, traceID func(string) string, tracer trace.Tracer) Handler {
	return Handler{repo: repo, log: log, now: now, traceID: traceID, tracer: tracer}
}

func (h Handler) Handle(ctx context.Context, query Query) (result Report, err error) {
	ctx, span := h.tracer.Start(ctx, spanReport)
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
	}()

	var run domain.Run
	if query.RunID != "" {
		run, err = h.repo.Get(ctx, query.RunID)
	} else {
		run, err = h.repo.Active(ctx, query.Key)
	}
	if err != nil {
		return Report{}, err
	}
	span.SetAttributes(attrRunID.String(run.ID))
	rows, err := h.log.Transitions(ctx, run.ID)
	if err != nil {
		return Report{}, err
	}
	result = Report{ID: run.ID, Phase: run.Phase, StopBlocks: run.StopBlocksTotal, TraceID: h.traceID(run.ID)}
	classes := map[domain.ReworkClass]int{}
	for _, row := range rows {
		switch {
		case row.Event.IsRework():
			result.ReworkRounds++
		case row.Event == domain.EventAmend:
			result.Amends++
		}
		if row.Class != "" {
			classes[row.Class]++
		}
	}
	result.Classes = sortedClasses(classes)
	result.Minutes = h.minutes(rows)
	return result, nil
}

func (h Handler) RepeatClasses(ctx context.Context, repo string) ([]domain.ClassCount, error) {
	return h.log.RepeatClasses(ctx, repo)
}

func (h Handler) minutes(rows []domain.Transitioned) []PhaseMinutes {
	var order []domain.Phase
	spent := map[domain.Phase]time.Duration{}
	current := domain.PhaseNone
	var since time.Time
	enter := func(phase domain.Phase, at time.Time) {
		if current != domain.PhaseNone {
			spent[current] += at.Sub(since)
		}
		if phase != domain.PhaseNone && !slices.Contains(order, phase) {
			order = append(order, phase)
		}
		current, since = phase, at
	}
	for _, row := range rows {
		enter(row.To, row.At)
	}
	enter(domain.PhaseNone, h.now())

	var minutes []PhaseMinutes
	for _, phase := range order {
		if phase == domain.PhaseDone || phase == domain.PhaseAbandoned {
			continue
		}
		minutes = append(minutes, PhaseMinutes{Phase: phase, Minutes: spent[phase].Minutes()})
	}
	return minutes
}

func sortedClasses(counts map[domain.ReworkClass]int) []domain.ClassCount {
	var classes []domain.ClassCount
	for class, count := range counts {
		classes = append(classes, domain.ClassCount{Class: class, Count: count})
	}
	slices.SortFunc(classes, func(a, b domain.ClassCount) int {
		if a.Count != b.Count {
			return b.Count - a.Count
		}
		return strings.Compare(string(a.Class), string(b.Class))
	})
	return classes
}
