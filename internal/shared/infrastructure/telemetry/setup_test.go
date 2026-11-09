package telemetry

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
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
	for _, name := range []string{EnvEndpoint, EnvTracesEndpoint, EnvMetricsEndpoint} {
		t.Setenv(name, "")
		t.Setenv(EnvAliasPrefix+name, "")
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
	t.Setenv(EnvEndpoint, c.server.URL)

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
	t.Setenv(EnvTracesEndpoint, c.server.URL+tracesPath)

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
	t.Setenv(EnvEndpoint, c.server.URL)

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
	t.Setenv(EnvEndpoint, c.server.URL)

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

func exportOnce(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	shutdown, err := Setup(ctx, testVersion)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	emitSignals(ctx, t)
	if err := shutdown(ctx); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}

func TestSetupHarnessSignalEndpoint(t *testing.T) {
	cases := []struct{ name, env, path, otherPath string }{
		{"traces", EnvTracesEndpoint, tracesPath, metricsPath},
		{"metrics", EnvMetricsEndpoint, metricsPath, tracesPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEndpointEnv(t)
			c := newCollector(t, okResponse)
			t.Setenv(EnvAliasPrefix+tc.env, c.server.URL+tc.path)

			exportOnce(t)

			if got := c.hitCount(tc.path); got != 1 {
				t.Errorf("%s requests = %d, want 1", tc.path, got)
			}
			if got := c.hitCount(tc.otherPath); got != 0 {
				t.Errorf("%s requests = %d, want 0", tc.otherPath, got)
			}
		})
	}
}

func TestSetupHarnessEndpoint(t *testing.T) {
	cases := []struct{ name, prefix string }{
		{"bare", ""},
		{"prefix", "/prefix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearEndpointEnv(t)
			c := newCollector(t, okResponse)
			t.Setenv(EnvAliasPrefix+EnvEndpoint, c.server.URL+tc.prefix)

			exportOnce(t)

			for _, path := range []string{tc.prefix + tracesPath, tc.prefix + metricsPath} {
				if got := c.hitCount(path); got != 1 {
					t.Errorf("%s requests = %d, want 1", path, got)
				}
			}
		})
	}
}

func TestLookupEndpointOrder(t *testing.T) {
	for _, signalEnv := range []string{EnvTracesEndpoint, EnvMetricsEndpoint} {
		t.Run(signalEnv, func(t *testing.T) {
			clearEndpointEnv(t)
			order := []string{EnvAliasPrefix + signalEnv, signalEnv, EnvAliasPrefix + EnvEndpoint, EnvEndpoint}
			for i, name := range order {
				t.Setenv(name, "http://host"+strconv.Itoa(i))
			}
			for i, want := range order {
				gotName, gotRaw := lookupEndpoint(signalEnv)
				if gotName != want || gotRaw != "http://host"+strconv.Itoa(i) {
					t.Errorf("slot %d = %q, %q, want %q", i, gotName, gotRaw, want)
				}
				t.Setenv(want, "")
			}
			if gotName, gotRaw := lookupEndpoint(signalEnv); gotName != "" || gotRaw != "" {
				t.Errorf("all clear = %q, %q, want empty", gotName, gotRaw)
			}
		})
	}
}

func TestSetupAliasWins(t *testing.T) {
	clearEndpointEnv(t)
	standard := newCollector(t, okResponse)
	alias := newCollector(t, okResponse)
	t.Setenv(EnvTracesEndpoint, standard.server.URL+tracesPath)
	t.Setenv(EnvAliasPrefix+EnvTracesEndpoint, alias.server.URL+tracesPath)

	exportOnce(t)

	if got := alias.hitCount(tracesPath); got != 1 {
		t.Errorf("alias trace requests = %d, want 1", got)
	}
	if got := standard.hitCount(tracesPath); got != 0 {
		t.Errorf("standard trace requests = %d, want 0", got)
	}
}

func TestSetupSignalBeatsGeneral(t *testing.T) {
	clearEndpointEnv(t)
	signal := newCollector(t, okResponse)
	general := newCollector(t, okResponse)
	t.Setenv(EnvAliasPrefix+EnvTracesEndpoint, signal.server.URL+tracesPath)
	t.Setenv(EnvEndpoint, general.server.URL)

	exportOnce(t)

	if got := signal.hitCount(tracesPath); got != 1 {
		t.Errorf("signal trace requests = %d, want 1", got)
	}
	if got := general.hitCount(tracesPath); got != 0 {
		t.Errorf("general trace requests = %d, want 0", got)
	}
}

func TestSetupInvalidEndpoint(t *testing.T) {
	const invalid = "localhost:4318"
	for _, name := range []string{EnvTracesEndpoint, EnvAliasPrefix + EnvTracesEndpoint} {
		t.Run(name, func(t *testing.T) {
			clearEndpointEnv(t)
			c := newCollector(t, okResponse)
			t.Setenv(name, invalid)
			t.Setenv(EnvMetricsEndpoint, c.server.URL+metricsPath)

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
				t.Fatal("exporters returned no error")
			}
			if !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), invalid) {
				t.Errorf("error = %q, want the name %q and the value %q", err, name, invalid)
			}
			if spans != nil {
				t.Errorf("span exporter = %v, want nil", spans)
			}
			if metrics == nil {
				t.Error("metric exporter is nil, want one for the valid endpoint")
			}
		})
	}
}
