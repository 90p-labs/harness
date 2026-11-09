package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	EnvTraceparent = "TRACEPARENT"
	keyTraceparent = "traceparent"
)

func TraceparentEntry(ctx context.Context) (string, bool) {
	if !trace.SpanContextFromContext(ctx).IsValid() {
		return "", false
	}
	carrier := propagation.MapCarrier{}
	propagation.TraceContext{}.Inject(ctx, carrier)
	return EnvTraceparent + "=" + carrier.Get(keyTraceparent), true
}

func ParentFromEnv(ctx context.Context) context.Context {
	carrier := propagation.MapCarrier{keyTraceparent: os.Getenv(EnvTraceparent)}
	return propagation.TraceContext{}.Extract(ctx, carrier)
}
