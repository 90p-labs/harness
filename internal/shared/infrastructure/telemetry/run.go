package telemetry

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"time"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

const EnvRunID = "HARNESS_RUN_ID"

const (
	AttrRunID     = "harness.run.id"
	AttrWorktree  = "harness.worktree"
	AttrRunActive = "harness.run.active"
	AttrRunPhase  = "harness.run.phase"
)

const SpanRun = "run"

type runKey struct{}

type rootIDsKey struct{}

func runSpanContext(runID string) trace.SpanContext {
	sum := sha256.Sum256([]byte(runID))
	return trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID(sum[0:16]),
		SpanID:     trace.SpanID(sum[16:24]),
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})
}

func TraceIDOf(runID string) string {
	return runSpanContext(runID).TraceID().String()
}

func UnderRun(ctx context.Context, runID string) context.Context {
	if runID == "" {
		return ctx
	}
	derived := runSpanContext(runID)
	if current := trace.SpanContextFromContext(ctx); current.IsValid() && current.TraceID() == derived.TraceID() {
		return ctx
	}
	return trace.ContextWithRemoteSpanContext(ctx, derived)
}

func EndRun(ctx context.Context, tracer trace.Tracer, runID, worktree, phase string, startedAt time.Time) {
	ctx = context.WithValue(ctx, rootIDsKey{}, runSpanContext(runID))
	_, span := tracer.Start(ctx, SpanRun,
		trace.WithNewRoot(),
		trace.WithTimestamp(startedAt),
		trace.WithAttributes(attribute.String(AttrRunID, runID), attribute.String(AttrWorktree, worktree), attribute.String(AttrRunPhase, phase)),
	)
	span.End()
}

type runIDGenerator struct{}

func (runIDGenerator) NewIDs(ctx context.Context) (trace.TraceID, trace.SpanID) {
	if root, ok := ctx.Value(rootIDsKey{}).(trace.SpanContext); ok {
		return root.TraceID(), root.SpanID()
	}
	var traceID trace.TraceID
	for traceID == (trace.TraceID{}) {
		_, _ = rand.Read(traceID[:])
	}
	return traceID, runIDGenerator{}.NewSpanID(ctx, traceID)
}

func (runIDGenerator) NewSpanID(_ context.Context, _ trace.TraceID) trace.SpanID {
	var spanID trace.SpanID
	for spanID == (trace.SpanID{}) {
		_, _ = rand.Read(spanID[:])
	}
	return spanID
}

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
