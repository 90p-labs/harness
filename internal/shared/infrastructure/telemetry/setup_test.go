package telemetry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

const (
	testVersion     = "9.9.9-test"
	tracesPath      = "/v1/traces"
	metricsPath     = "/v1/metrics"
	resourceService = "service.name"
	resourceVersion = "service.version"
	wantServiceName = "harness"
	deadlineSlack   = time.Second
)

type collector struct {
	server  *httptest.Server
	mu      sync.Mutex
	hits    map[string]int
	traces  []*collectortrace.ExportTraceServiceRequest
	metrics []*collectormetrics.ExportMetricsServiceRequest
}

func newCollector(t *testing.T, respond func(http.ResponseWriter, *http.Request)) *collector {
	t.Helper()
	c := &collector{hits: map[string]int{}}
	c.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		c.record(r.URL.Path, body, t)
		respond(w, r)
	}))
	t.Cleanup(c.server.Close)
	return c
}

func (c *collector) record(path string, body []byte, t *testing.T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hits[path]++
	switch path {
	case tracesPath:
		req := &collectortrace.ExportTraceServiceRequest{}
		if err := proto.Unmarshal(body, req); err == nil {
			c.traces = append(c.traces, req)
		}
	case metricsPath:
		req := &collectormetrics.ExportMetricsServiceRequest{}
		if err := proto.Unmarshal(body, req); err == nil {
			c.metrics = append(c.metrics, req)
		}
	}
}

func (c *collector) hitCount(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits[path]
}

func okResponse(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func clearEndpointEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{envEndpoint, envTracesEndpoint, envMetricsEndpoint} {
		t.Setenv(name, "")
	}
}

func emitSignals(ctx context.Context, t *testing.T) {
	t.Helper()
	_, span := otel.Tracer("telemetry-test").Start(ctx, "operation")
	span.End()
	counter, err := otel.Meter("telemetry-test").Int64Counter("operations")
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	counter.Add(ctx, 1)
}

func resourceAttr(res *resourcev1.Resource, key string) string {
	for _, kv := range res.GetAttributes() {
		if kv.GetKey() == key {
			return kv.GetValue().GetStringValue()
		}
	}
	return ""
}

func TestSetupExports(t *testing.T) {
	clearEndpointEnv(t)
	c := newCollector(t, okResponse)
	t.Setenv(envEndpoint, c.server.URL)

	ctx := context.Background()
	shutdown, err := Setup(ctx, testVersion)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	emitSignals(ctx, t)
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if len(c.traces) != 1 || len(c.metrics) != 1 {
		t.Fatalf("got %d trace and %d metric requests, want 1 each", len(c.traces), len(c.metrics))
	}
	traceRes := c.traces[0].GetResourceSpans()[0].GetResource()
	metricRes := c.metrics[0].GetResourceMetrics()[0].GetResource()
	for name, res := range map[string]*resourcev1.Resource{"traces": traceRes, "metrics": metricRes} {
		if got := resourceAttr(res, resourceService); got != wantServiceName {
			t.Errorf("%s service.name = %q, want %q", name, got, wantServiceName)
		}
		if got := resourceAttr(res, resourceVersion); got != testVersion {
			t.Errorf("%s service.version = %q, want %q", name, got, testVersion)
		}
	}
}

func TestSetupNoEndpoint(t *testing.T) {
	clearEndpointEnv(t)

	shutdown, err := Setup(context.Background(), testVersion)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Errorf("tracer provider is %T, want the SDK type", otel.GetTracerProvider())
	}
	if _, ok := otel.GetMeterProvider().(*sdkmetric.MeterProvider); !ok {
		t.Errorf("meter provider is %T, want the SDK type", otel.GetMeterProvider())
	}
	spans, metrics, err := exporters(context.Background())
	if err != nil || spans != nil || metrics != nil {
		t.Errorf("exporters = %v, %v, %v; want nil, nil, nil", spans, metrics, err)
	}
}

func TestSetupSignalEndpoint(t *testing.T) {
	clearEndpointEnv(t)
	c := newCollector(t, okResponse)
	t.Setenv(envTracesEndpoint, c.server.URL+tracesPath)

	ctx := context.Background()
	shutdown, err := Setup(ctx, testVersion)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	emitSignals(ctx, t)
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if got := c.hitCount(tracesPath); got != 1 {
		t.Errorf("trace requests = %d, want 1", got)
	}
	if got := c.hitCount(metricsPath); got != 0 {
		t.Errorf("metric requests = %d, want 0", got)
	}
}

func TestExportNoRetry(t *testing.T) {
	clearEndpointEnv(t)
	c := newCollector(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	t.Setenv(envEndpoint, c.server.URL)

	ctx := context.Background()
	shutdown, err := Setup(ctx, testVersion)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	emitSignals(ctx, t)
	_ = shutdown(ctx)

	for _, path := range []string{tracesPath, metricsPath} {
		if got := c.hitCount(path); got != 1 {
			t.Errorf("%s requests = %d, want 1", path, got)
		}
	}
}

func TestShutdownDeadline(t *testing.T) {
	clearEndpointEnv(t)
	release := make(chan struct{})
	c := newCollector(t, func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	t.Cleanup(func() { close(release) })
	t.Setenv(envEndpoint, c.server.URL)

	ctx := context.Background()
	shutdown, err := Setup(ctx, testVersion)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	emitSignals(ctx, t)

	start := time.Now()
	err = shutdown(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Error("shutdown returned no error, want one")
	}
	if elapsed > ShutdownTimeout+deadlineSlack {
		t.Errorf("shutdown took %v, want at most %v", elapsed, ShutdownTimeout+deadlineSlack)
	}
}

func TestSetupInvalidEndpoint(t *testing.T) {
	clearEndpointEnv(t)
	c := newCollector(t, okResponse)
	t.Setenv(envTracesEndpoint, "localhost:4318")
	t.Setenv(envMetricsEndpoint, c.server.URL+metricsPath)

	shutdown, err := Setup(context.Background(), testVersion)
	if err == nil {
		t.Fatal("Setup returned no error for an endpoint without a scheme")
	}
	if shutdown == nil {
		t.Fatal("Setup returned a nil shutdown with an error")
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	spans, metrics, err := exporters(context.Background())
	if err == nil {
		t.Error("exporters returned no error")
	}
	if spans != nil {
		t.Errorf("span exporter = %v, want nil", spans)
	}
	if metrics == nil {
		t.Error("metric exporter is nil, want one for the valid endpoint")
	}
}
