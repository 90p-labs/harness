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
	envEndpoint        = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envTracesEndpoint  = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	envMetricsEndpoint = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
)

const (
	attrServiceName    = "service.name"
	attrServiceVersion = "service.version"
	serviceName        = "harness"
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

	if endpoint := signalEndpoint(envTracesEndpoint); endpoint != "" {
		if err := validateEndpoint(envTracesEndpoint, endpoint); err != nil {
			errs = append(errs, err)
		} else if exporter, err := otlptracehttp.New(ctx,
			otlptracehttp.WithRetry(otlptracehttp.RetryConfig{}),
			otlptracehttp.WithTimeout(ShutdownTimeout),
		); err != nil {
			errs = append(errs, err)
		} else {
			spanExporter = exporter
		}
	}

	if endpoint := signalEndpoint(envMetricsEndpoint); endpoint != "" {
		if err := validateEndpoint(envMetricsEndpoint, endpoint); err != nil {
			errs = append(errs, err)
		} else if exporter, err := otlpmetrichttp.New(ctx,
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

func signalEndpoint(signalEnv string) string {
	if endpoint := os.Getenv(signalEnv); endpoint != "" {
		return endpoint
	}
	return os.Getenv(envEndpoint)
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
