package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	envTraceparent = "TRACEPARENT"
	keyTraceparent = "traceparent"
)

func TraceparentEntry(ctx context.Context) (string, bool) {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return "", false
	}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return envTraceparent + "=" + carrier.Get(keyTraceparent), true
}

func ParentFromEnv(ctx context.Context) context.Context {
	carrier := propagation.MapCarrier{keyTraceparent: os.Getenv(envTraceparent)}
	return propagation.TraceContext{}.Extract(ctx, carrier)
}
