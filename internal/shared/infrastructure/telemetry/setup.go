package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const ShutdownTimeout = 2 * time.Second

const (
	EnvAliasPrefix     = "HARNESS_"
	EnvEndpoint        = "OTEL_EXPORTER_OTLP_ENDPOINT"
	EnvTracesEndpoint  = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	EnvMetricsEndpoint = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
)

const (
	attrServiceName    = "service.name"
	attrServiceVersion = "service.version"
	serviceName        = "harness"
)

const (
	tracesPath  = "/v1/traces"
	metricsPath = "/v1/metrics"
)

const (
	schemeHTTP  = "http"
	schemeHTTPS = "https"
)

func Setup(ctx context.Context, version string) (func(context.Context) error, error) {
	spanExporter, metricExporter, err := exporters(ctx)

	res := resource.NewSchemaless(
		attribute.String(attrServiceName, serviceName),
		attribute.String(attrServiceVersion, version),
	)

	traceOptions := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if spanExporter != nil {
		traceOptions = append(traceOptions, sdktrace.WithBatcher(spanExporter))
	}
	tracerProvider := sdktrace.NewTracerProvider(traceOptions...)

	meterOptions := []sdkmetric.Option{sdkmetric.WithResource(res)}
	if metricExporter != nil {
		meterOptions = append(meterOptions, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)))
	}
	meterProvider := sdkmetric.NewMeterProvider(meterOptions...)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)

	shutdown := func(ctx context.Context) error {
		ctx, cancel := context.WithTimeout(ctx, ShutdownTimeout)
		defer cancel()
		return errors.Join(tracerProvider.Shutdown(ctx), meterProvider.Shutdown(ctx))
	}
	return shutdown, err
}

func exporters(ctx context.Context) (sdktrace.SpanExporter, sdkmetric.Exporter, error) {
	var spanExporter sdktrace.SpanExporter
	var metricExporter sdkmetric.Exporter
	var errs []error

	if endpoint, err := resolveEndpoint(EnvTracesEndpoint, tracesPath); err != nil {
		errs = append(errs, err)
	} else if endpoint != "" {
		if exporter, err := otlptracehttp.New(ctx,
			otlptracehttp.WithEndpointURL(endpoint),
			otlptracehttp.WithRetry(otlptracehttp.RetryConfig{}),
			otlptracehttp.WithTimeout(ShutdownTimeout),
		); err != nil {
			errs = append(errs, err)
		} else {
			spanExporter = exporter
		}
	}

	if endpoint, err := resolveEndpoint(EnvMetricsEndpoint, metricsPath); err != nil {
		errs = append(errs, err)
	} else if endpoint != "" {
		if exporter, err := otlpmetrichttp.New(ctx,
			otlpmetrichttp.WithEndpointURL(endpoint),
			otlpmetrichttp.WithRetry(otlpmetrichttp.RetryConfig{}),
			otlpmetrichttp.WithTimeout(ShutdownTimeout),
		); err != nil {
			errs = append(errs, err)
		} else {
			metricExporter = exporter
		}
	}

	return spanExporter, metricExporter, errors.Join(errs...)
}

func resolveEndpoint(signalEnv, signalPath string) (string, error) {
	name, raw := lookupEndpoint(signalEnv)
	if raw == "" {
		return "", nil
	}
	if err := validateEndpoint(name, raw); err != nil {
		return "", err
	}
	if name == signalEnv || name == EnvAliasPrefix+signalEnv {
		return raw, nil
	}
	joined, err := url.JoinPath(raw, signalPath)
	if err != nil {
		return "", fmt.Errorf("%s: %w", name, err)
	}
	return joined, nil
}

func lookupEndpoint(signalEnv string) (string, string) {
	for _, name := range []string{EnvAliasPrefix + signalEnv, signalEnv, EnvAliasPrefix + EnvEndpoint, EnvEndpoint} {
		if raw := os.Getenv(name); raw != "" {
			return name, raw
		}
	}
	return "", ""
}

func validateEndpoint(name, endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	if (parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS) || parsed.Host == "" {
		return fmt.Errorf("%s: %q needs an http or https scheme and a host", name, endpoint)
	}
	return nil
}
