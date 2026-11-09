package telemetry

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

const (
	testTraceID      = "0af7651916cd43dd8448eb211c80319c"
	testSpanID       = "b7ad6b7169203331"
	traceparentValid = "00-" + testTraceID + "-" + testSpanID + "-01"
	traceparentOff   = "00-" + testTraceID + "-" + testSpanID + "-00"
)

func TestParentFromEnv(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		wantParent  bool
	}{
		{"valid", traceparentValid, true},
		{"empty", "", false},
		{"bad", "not-a-traceparent", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvTraceparent, tc.value)

			got := trace.SpanContextFromContext(ParentFromEnv(context.Background()))

			if got.IsValid() != tc.wantParent {
				t.Fatalf("valid = %v, want %v", got.IsValid(), tc.wantParent)
			}
			if tc.wantParent && got.TraceID().String() != testTraceID {
				t.Errorf("trace id = %s, want %s", got.TraceID(), testTraceID)
			}
		})
	}
}

func TestParentFromEnvFlags(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		wantRecord  bool
	}{
		{"flag 01 keeps the span", traceparentValid, true},
		{"flag 00 drops the span", traceparentOff, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(EnvTraceparent, tc.value)
			recorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))

			ctx, span := provider.Tracer("test").Start(ParentFromEnv(context.Background()), "child")
			span.End()

			if got := len(recorder.Ended()) == 1; got != tc.wantRecord {
				t.Errorf("recorded = %v, want %v", got, tc.wantRecord)
			}
			if got := trace.SpanContextFromContext(ctx).TraceID().String(); got != testTraceID {
				t.Errorf("child trace id = %s, want %s", got, testTraceID)
			}
		})
	}
}

func TestChildEnvTraceparent(t *testing.T) {
	t.Setenv(EnvRunID, "")
	if got := ChildEnv(context.Background()); len(got) != 0 {
		t.Fatalf("ChildEnv = %v for a context with no span", got)
	}
	t.Setenv(EnvTraceparent, traceparentValid)

	got := ChildEnv(ParentFromEnv(context.Background()))

	if want := EnvTraceparent + "=" + traceparentValid; len(got) != 1 || got[0] != want {
		t.Fatalf("ChildEnv = %v, want [%q]", got, want)
	}
}
