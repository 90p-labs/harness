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

func ChildEnv(ctx context.Context) []string {
	var env []string
	if trace.SpanContextFromContext(ctx).IsValid() {
		carrier := propagation.MapCarrier{}
		propagation.TraceContext{}.Inject(ctx, carrier)
		env = append(env, EnvTraceparent+"="+carrier.Get(keyTraceparent))
	}
	if id := runOfSpan(trace.SpanFromContext(ctx)).or(runFromContext(ctx)).id; id != "" {
		env = append(env, EnvRunID+"="+id)
	}
	return env
}

func ParentFromEnv(ctx context.Context) context.Context {
	carrier := propagation.MapCarrier{keyTraceparent: os.Getenv(EnvTraceparent)}
	ctx = propagation.TraceContext{}.Extract(ctx, carrier)
	if id := os.Getenv(EnvRunID); id != "" {
		ctx = WithRun(ctx, "", id)
	}
	return ctx
}
