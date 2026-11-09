package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const (
	testRunID    = "run-1"
	otherRunID   = "run-2"
	testWorktree = "/work/a"
	otherTree    = "/work/b"
)

func stampedTracer() (trace.Tracer, *tracetest.SpanRecorder) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(runStamp{}), sdktrace.WithSpanProcessor(recorder))
	return provider.Tracer("test"), recorder
}

func attrsOf(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	got := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Attributes() {
		got[kv.Key] = kv.Value
	}
	return got
}

func endedAttrs(t *testing.T, recorder *tracetest.SpanRecorder) map[attribute.Key]attribute.Value {
	t.Helper()
	ended := recorder.Ended()
	if len(ended) != 1 {
		t.Fatalf("ended spans = %d, want 1", len(ended))
	}
	return attrsOf(ended[0])
}

func TestRunStampKeepsOwnRunID(t *testing.T) {
	tracer, recorder := stampedTracer()
	ctx := WithRun(context.Background(), testWorktree, testRunID)

	_, span := tracer.Start(ctx, "s", trace.WithAttributes(attribute.String(AttrRunID, otherRunID)))
	span.End()

	got := endedAttrs(t, recorder)
	if got[AttrRunID].AsString() != otherRunID {
		t.Errorf("run id = %q, want %q", got[AttrRunID].AsString(), otherRunID)
	}
}

func TestRunStampChildInheritsParent(t *testing.T) {
	tracer, recorder := stampedTracer()
	parentCtx, parent := tracer.Start(context.Background(), "parent", trace.WithAttributes(
		attribute.String(AttrRunID, testRunID), attribute.String(AttrWorktree, testWorktree)))

	_, child := tracer.Start(parentCtx, "child")
	child.End()
	parent.End()

	got := attrsOf(recorder.Ended()[0])
	if got[AttrRunID].AsString() != testRunID || got[AttrWorktree].AsString() != testWorktree {
		t.Errorf("child attrs = %v, want run %q worktree %q", got, testRunID, testWorktree)
	}
	if _, ok := got[AttrRunActive]; ok {
		t.Errorf("child has %s, want none", AttrRunActive)
	}
}

func TestRunStampParentSpanBeatsContextRun(t *testing.T) {
	tracer, recorder := stampedTracer()
	parentCtx, parent := tracer.Start(context.Background(), "parent", trace.WithAttributes(
		attribute.String(AttrRunID, testRunID), attribute.String(AttrWorktree, testWorktree)))

	_, child := tracer.Start(WithRun(parentCtx, otherTree, otherRunID), "child")
	child.End()
	parent.End()

	got := attrsOf(recorder.Ended()[0])
	if got[AttrRunID].AsString() != testRunID || got[AttrWorktree].AsString() != testWorktree {
		t.Errorf("child attrs = %v, want run %q worktree %q", got, testRunID, testWorktree)
	}
	if _, ok := got[AttrRunActive]; ok {
		t.Errorf("child has %s, want none", AttrRunActive)
	}
}

func TestRunStampRootTakesContextRun(t *testing.T) {
	tracer, recorder := stampedTracer()

	_, span := tracer.Start(WithRun(context.Background(), testWorktree, testRunID), "root")
	span.End()

	got := endedAttrs(t, recorder)
	if got[AttrRunID].AsString() != testRunID || got[AttrWorktree].AsString() != testWorktree {
		t.Errorf("attrs = %v, want run %q worktree %q", got, testRunID, testWorktree)
	}
}

func TestRunStampRemoteParentFallsBackToContextRun(t *testing.T) {
	t.Setenv(EnvTraceparent, traceparentValid)
	t.Setenv(EnvRunID, testRunID)
	tracer, recorder := stampedTracer()

	_, span := tracer.Start(ParentFromEnv(context.Background()), "root")
	span.End()

	if got := endedAttrs(t, recorder); got[AttrRunID].AsString() != testRunID {
		t.Errorf("run id = %v, want %q", got[AttrRunID], testRunID)
	}
}

func TestRunStampNoRunMarksInactive(t *testing.T) {
	tracer, recorder := stampedTracer()

	_, span := tracer.Start(context.Background(), "root")
	span.End()

	got := endedAttrs(t, recorder)
	if v, ok := got[AttrRunActive]; !ok || v.AsBool() {
		t.Errorf("%s = %v, %v, want false", AttrRunActive, v, ok)
	}
	if _, ok := got[AttrRunID]; ok {
		t.Errorf("%s present, want none", AttrRunID)
	}
}

func TestRunStampWorktreeOnly(t *testing.T) {
	tracer, recorder := stampedTracer()

	_, span := tracer.Start(WithRun(context.Background(), testWorktree, ""), "root")
	span.End()

	got := endedAttrs(t, recorder)
	if got[AttrWorktree].AsString() != testWorktree {
		t.Errorf("worktree = %q, want %q", got[AttrWorktree].AsString(), testWorktree)
	}
	if v, ok := got[AttrRunActive]; !ok || v.AsBool() {
		t.Errorf("%s = %v, %v, want false", AttrRunActive, v, ok)
	}
	if _, ok := got[AttrRunID]; ok {
		t.Errorf("%s present, want none", AttrRunID)
	}
}

func TestRunStampNeverOverwritesExplicitWorktree(t *testing.T) {
	tracer, recorder := stampedTracer()

	_, span := tracer.Start(WithRun(context.Background(), testWorktree, testRunID), "gate.verify",
		trace.WithAttributes(attribute.String(AttrWorktree, otherTree)))
	span.End()

	got := endedAttrs(t, recorder)
	if got[AttrWorktree].AsString() != otherTree || got[AttrRunID].AsString() != testRunID {
		t.Errorf("attrs = %v, want worktree %q kept and run %q", got, otherTree, testRunID)
	}
}

func TestRunEnvChildEnvCarriesBoth(t *testing.T) {
	t.Setenv(EnvTraceparent, traceparentValid)
	t.Setenv(EnvRunID, testRunID)

	got := ChildEnv(ParentFromEnv(context.Background()))

	want := []string{EnvTraceparent + "=" + traceparentValid, EnvRunID + "=" + testRunID}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("ChildEnv = %v, want %v", got, want)
	}
}

func TestRunEnvSpanRunWinsOverContextRun(t *testing.T) {
	tracer, _ := stampedTracer()
	ctx, span := tracer.Start(WithRun(context.Background(), testWorktree, testRunID), "s",
		trace.WithAttributes(attribute.String(AttrRunID, otherRunID)))
	defer span.End()

	got := ChildEnv(ctx)

	if len(got) != 2 || got[1] != EnvRunID+"="+otherRunID {
		t.Fatalf("ChildEnv = %v, want run %q", got, otherRunID)
	}
}

func TestRunEnvEmptyContextHasNoEntries(t *testing.T) {
	if got := ChildEnv(context.Background()); len(got) != 0 {
		t.Fatalf("ChildEnv = %v, want none", got)
	}
}

func TestRunEnvParentFromEnvSpansCarryRun(t *testing.T) {
	t.Setenv(EnvRunID, testRunID)
	tracer, recorder := stampedTracer()

	_, span := tracer.Start(ParentFromEnv(context.Background()), "s")
	span.End()

	if got := endedAttrs(t, recorder); got[AttrRunID].AsString() != testRunID {
		t.Errorf("run id = %v, want %q", got[AttrRunID], testRunID)
	}
}

func TestRunEnvWithRunEmptyIDKeepsEnvRun(t *testing.T) {
	t.Setenv(EnvRunID, testRunID)
	tracer, recorder := stampedTracer()

	_, span := tracer.Start(WithRun(ParentFromEnv(context.Background()), testWorktree, ""), "s")
	span.End()

	got := endedAttrs(t, recorder)
	if got[AttrRunID].AsString() != testRunID || got[AttrWorktree].AsString() != testWorktree {
		t.Errorf("attrs = %v, want run %q worktree %q", got, testRunID, testWorktree)
	}
}
