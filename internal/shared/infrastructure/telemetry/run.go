package telemetry

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const EnvRunID = "HARNESS_RUN_ID"

const (
	AttrRunID     = "harness.run.id"
	AttrWorktree  = "harness.worktree"
	AttrRunActive = "harness.run.active"
)

type runKey struct{}

type run struct {
	worktree, id string
}

func WithRun(ctx context.Context, worktree, runID string) context.Context {
	held := runFromContext(ctx)
	if worktree != "" {
		held.worktree = worktree
	}
	if runID != "" {
		held.id = runID
	}
	return context.WithValue(ctx, runKey{}, held)
}

func runFromContext(ctx context.Context) run {
	held, _ := ctx.Value(runKey{}).(run)
	return held
}

func runOfSpan(span trace.Span) run {
	readable, ok := span.(sdktrace.ReadOnlySpan)
	if !ok {
		return run{}
	}
	return runOfAttributes(readable.Attributes())
}

func runOfAttributes(attrs []attribute.KeyValue) run {
	var found run
	for _, kv := range attrs {
		switch kv.Key {
		case AttrRunID:
			found.id = kv.Value.AsString()
		case AttrWorktree:
			found.worktree = kv.Value.AsString()
		}
	}
	return found
}

func (r run) or(fallback run) run {
	if r.id == "" {
		r.id = fallback.id
	}
	if r.worktree == "" {
		r.worktree = fallback.worktree
	}
	return r
}

type runStamp struct{}

func (runStamp) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	own := runOfAttributes(s.Attributes())
	resolved := own.or(runOfSpan(trace.SpanFromContext(parent))).or(runFromContext(parent))
	if own.id == "" {
		if resolved.id != "" {
			s.SetAttributes(attribute.String(AttrRunID, resolved.id))
		} else {
			s.SetAttributes(attribute.Bool(AttrRunActive, false))
		}
	}
	if own.worktree == "" && resolved.worktree != "" {
		s.SetAttributes(attribute.String(AttrWorktree, resolved.worktree))
	}
}

func (runStamp) OnEnd(sdktrace.ReadOnlySpan)      {}
func (runStamp) Shutdown(context.Context) error   { return nil }
func (runStamp) ForceFlush(context.Context) error { return nil }
