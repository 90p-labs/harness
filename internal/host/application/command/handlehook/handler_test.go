package handlehook_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/jorge07/harness/internal/host/application/command/handlehook"
	hostdomain "github.com/jorge07/harness/internal/host/domain"
	"github.com/jorge07/harness/internal/host/infrastructure/hookcodec"
	"github.com/jorge07/harness/internal/shared/application/eventbus"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	preToolInput      = `{"cwd":"/work","tool_name":"Bash","tool_input":{"command":"ls"}}`
	sessionInput      = `{"cwd":"/work","source":"startup"}`
	firstStopInput    = `{"cwd":"/work","stop_hook_active":false}`
	retriedStopInput  = `{"cwd":"/work","stop_hook_active":true}`
	retriedBadStop    = `{"cwd":"/work","stop_hook_active":true,"background_tasks":{"id":"x"}}`
	errBoom           = "boom"
	harnessErrorText  = "harness error"
	denyDecisionText  = `"permissionDecision":"deny"`
	blockDecisionText = `"decision":"block"`
	codexAllowOnStop  = "{}"
)

type dispatcherFunc func(context.Context, shareddomain.Envelope) (shareddomain.DispatchResult, error)

func (f dispatcherFunc) Dispatch(ctx context.Context, e shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	return f(ctx, e)
}

func connectTo(d shareddomain.Dispatcher) handlehook.Connect {
	return func(context.Context) (shareddomain.Dispatcher, error) { return d, nil }
}

func allowAll() shareddomain.Dispatcher {
	return dispatcherFunc(func(context.Context, shareddomain.Envelope) (shareddomain.DispatchResult, error) {
		return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictAllow}}, nil
	})
}

func failing() shareddomain.Dispatcher {
	return dispatcherFunc(func(context.Context, shareddomain.Envelope) (shareddomain.DispatchResult, error) {
		return shareddomain.DispatchResult{}, errors.New(errBoom)
	})
}

func newHandler(t *testing.T, connect handlehook.Connect, codecs ...hostdomain.Codec) handlehook.Handler {
	t.Helper()
	tracer := sdktrace.NewTracerProvider().Tracer("test")
	meter := sdkmetric.NewMeterProvider().Meter("test")
	return handlehook.NewHandler(codecs, connect, tracer, meter)
}

type partialCodec struct{}

func (partialCodec) Host() shareddomain.Host { return shareddomain.HostCodex }

func (partialCodec) Capabilities() []shareddomain.EventName {
	return []shareddomain.EventName{
		shareddomain.EventSessionStarted,
		shareddomain.EventPromptSubmitted,
		shareddomain.EventToolBefore,
		shareddomain.EventTurnStopping,
	}
}

func (partialCodec) Decode(_ hostdomain.HookName, raw []byte) (shareddomain.Envelope, error) {
	payload, err := json.Marshal(shareddomain.SessionPayload{Source: string(raw)})
	return shareddomain.Envelope{Event: shareddomain.EventSessionStarted, Payload: payload}, err
}

func (partialCodec) Encode(_ hostdomain.HookName, v shareddomain.Verdict) ([]byte, error) {
	return []byte(v.Context), nil
}

func TestHookFailsClosed(t *testing.T) {
	cases := []struct {
		name       string
		codec      hostdomain.Codec
		hook       hostdomain.HookName
		input      string
		connect    handlehook.Connect
		wantDeny   bool
		wantStdout string
	}{
		{name: "veto event with a dispatch error", codec: hookcodec.NewClaude(), hook: hostdomain.HookPreToolUse, input: preToolInput, connect: connectTo(failing()), wantDeny: true},
		{name: "veto event with a decode error", codec: hookcodec.NewClaude(), hook: hostdomain.HookPreToolUse, input: "", connect: connectTo(allowAll()), wantDeny: true},
		{name: "veto event with a connect error", codec: hookcodec.NewClaude(), hook: hostdomain.HookPreToolUse, input: preToolInput,
			connect: func(context.Context) (shareddomain.Dispatcher, error) { return nil, errors.New(errBoom) }, wantDeny: true},
		{name: "context event on claude", codec: hookcodec.NewClaude(), hook: hostdomain.HookPostToolUse, input: "", connect: connectTo(allowAll())},
		{name: "stop on codex blocks", codec: hookcodec.NewCodex(), hook: hostdomain.HookStop, input: "", connect: connectTo(allowAll()), wantDeny: true},
		{name: "subagent stop on codex", codec: hookcodec.NewCodex(), hook: hostdomain.HookSubagentStop, input: "", connect: connectTo(allowAll()), wantStdout: codexAllowOnStop},
		{name: "first stop with a dispatch error blocks", codec: hookcodec.NewClaude(), hook: hostdomain.HookStop, input: firstStopInput, connect: connectTo(failing()), wantDeny: true},
		{name: "stop after a block allows", codec: hookcodec.NewClaude(), hook: hostdomain.HookStop, input: retriedStopInput, connect: connectTo(failing())},
		{name: "stop after a block on codex", codec: hookcodec.NewCodex(), hook: hostdomain.HookStop, input: retriedStopInput, connect: connectTo(failing()), wantStdout: codexAllowOnStop},
		{name: "stop after a block with a decode error", codec: hookcodec.NewClaude(), hook: hostdomain.HookStop, input: retriedBadStop, connect: connectTo(allowAll())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler := newHandler(t, tc.connect, tc.codec)
			out := handler.Handle(context.Background(), handlehook.Command{
				Host: tc.codec.Host(), Hook: tc.hook, Input: strings.NewReader(tc.input),
			})

			if tc.wantDeny {
				mark := denyDecisionText
				if tc.hook == hostdomain.HookStop {
					mark = blockDecisionText
				}
				if !strings.Contains(string(out.Stdout), mark) || !strings.Contains(string(out.Stdout), harnessErrorText) {
					t.Fatalf("stdout = %q, want %s with %q", out.Stdout, mark, harnessErrorText)
				}
				if out.Stderr != "" {
					t.Fatalf("stderr = %q, want empty", out.Stderr)
				}
				return
			}
			if string(out.Stdout) != tc.wantStdout {
				t.Fatalf("stdout = %q, want %q", out.Stdout, tc.wantStdout)
			}
			if !strings.HasPrefix(out.Stderr, harnessErrorText+": ") || strings.Count(out.Stderr, "\n") != 1 || !strings.HasSuffix(out.Stderr, "\n") {
				t.Fatalf("stderr = %q, want one %q line", out.Stderr, harnessErrorText)
			}
		})
	}
}

func TestHookDeniesFromTheBus(t *testing.T) {
	deny := dispatcherFunc(func(context.Context, shareddomain.Envelope) (shareddomain.DispatchResult, error) {
		return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "not now"}}, nil
	})
	handler := newHandler(t, connectTo(deny), hookcodec.NewClaude())

	out := handler.Handle(context.Background(), handlehook.Command{
		Host: shareddomain.HostClaude, Hook: hostdomain.HookPreToolUse, Input: strings.NewReader(preToolInput),
	})

	if !strings.Contains(string(out.Stdout), denyDecisionText) || !strings.Contains(string(out.Stdout), "not now") || out.Stderr != "" {
		t.Fatalf("out = %q, %q, want the bus deny", out.Stdout, out.Stderr)
	}
}

func TestRetriedStopWithBusVerdict(t *testing.T) {
	const pluginName = "guard"
	cases := []struct {
		name       string
		result     shareddomain.DispatchResult
		wantStdout string
		wantStderr string
	}{
		{
			name: "plugin failure allows",
			result: shareddomain.DispatchResult{
				Verdict:  shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "plugin failed: " + errBoom},
				Failures: []shareddomain.DispatchFailure{{Plugin: pluginName, Err: errors.New(errBoom)}},
			},
			wantStderr: harnessErrorText + ": " + pluginName + ": " + errBoom + "\n",
		},
		{
			name:       "plugin deny blocks",
			result:     shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "tests fail"}},
			wantStdout: `{"decision":"block","reason":"tests fail"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bus := dispatcherFunc(func(context.Context, shareddomain.Envelope) (shareddomain.DispatchResult, error) {
				return tc.result, nil
			})
			handler := newHandler(t, connectTo(bus), hookcodec.NewClaude())

			out := handler.Handle(context.Background(), handlehook.Command{
				Host: shareddomain.HostClaude, Hook: hostdomain.HookStop, Input: strings.NewReader(retriedStopInput),
			})

			if string(out.Stdout) != tc.wantStdout || out.Stderr != tc.wantStderr {
				t.Fatalf("out = %q, %q, want %q, %q", out.Stdout, out.Stderr, tc.wantStdout, tc.wantStderr)
			}
		})
	}
}

func TestMissingEventFallback(t *testing.T) {
	var seen shareddomain.Envelope
	spy := dispatcherFunc(func(_ context.Context, e shareddomain.Envelope) (shareddomain.DispatchResult, error) {
		seen = e
		return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictContext, Context: "from plugin"}}, nil
	})
	handler := newHandler(t, connectTo(spy), partialCodec{})

	out := handler.Handle(context.Background(), handlehook.Command{
		Host: shareddomain.HostCodex, Hook: hostdomain.HookSessionStart, Input: strings.NewReader(sessionInput),
	})

	var payload shareddomain.SessionPayload
	if err := json.Unmarshal(seen.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	wantMissing := []shareddomain.EventName{
		shareddomain.EventToolAfter, shareddomain.EventSubagentStarted, shareddomain.EventSubagentStopped,
	}
	if !slices.Equal(payload.MissingEvents, wantMissing) {
		t.Fatalf("missing_events = %v, want %v", payload.MissingEvents, wantMissing)
	}
	if payload.Source != sessionInput {
		t.Fatalf("source = %q, want the decoded source kept", payload.Source)
	}
	lines := strings.Split(string(out.Stdout), "\n")
	if lines[0] != "from plugin" {
		t.Fatalf("context = %q, want the plugin context first", out.Stdout)
	}
	var fallback []string
	for _, line := range lines {
		if strings.HasSuffix(line, ": this host does not send it") {
			fallback = append(fallback, line)
		}
	}
	want := []string{
		"tool.after: this host does not send it",
		"subagent.started: this host does not send it",
		"subagent.stopped: this host does not send it",
	}
	if !slices.Equal(fallback, want) {
		t.Fatalf("fallback lines = %q, want %q", fallback, want)
	}
}

func TestHookInputLimit(t *testing.T) {
	const limit = 16 << 20
	prefix := `{"cwd":"/work","tool_name":"Bash","tool_input":{"command":"`
	suffix := `"}}`
	build := func(size int) string {
		return prefix + strings.Repeat("a", size-len(prefix)-len(suffix)) + suffix
	}
	handler := newHandler(t, connectTo(allowAll()), hookcodec.NewClaude())
	run := func(input string) handlehook.Output {
		return handler.Handle(context.Background(), handlehook.Command{
			Host: shareddomain.HostClaude, Hook: hostdomain.HookPreToolUse, Input: strings.NewReader(input),
		})
	}

	over := run(build(limit + 1))
	if !strings.Contains(string(over.Stdout), "input above 16 MiB") {
		t.Fatalf("stdout = %.200q, want a deny naming the limit", over.Stdout)
	}
	exact := run(build(limit))
	if len(exact.Stdout) != 0 || exact.Stderr != "" {
		t.Fatalf("input of exactly 16 MiB: stdout = %.200q, stderr = %q, want an allow", exact.Stdout, exact.Stderr)
	}
}

func TestHandleHookTelemetry(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	reader := sdkmetric.NewManualReader()
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)).Meter("test")
	handler := handlehook.NewHandler([]hostdomain.Codec{hookcodec.NewClaude()}, connectTo(allowAll()), tracer, meter)

	handler.Handle(context.Background(), handlehook.Command{
		Host: shareddomain.HostClaude, Hook: hostdomain.HookPreToolUse, Input: strings.NewReader(preToolInput),
	})

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "hook.handle" {
		t.Fatalf("spans = %d, want one hook.handle", len(spans))
	}
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	var points []metricdata.HistogramDataPoint[float64]
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if m.Name != "harness.hook.duration" {
				continue
			}
			points = append(points, m.Data.(metricdata.Histogram[float64]).DataPoints...)
		}
	}
	if len(points) != 1 || points[0].Count != 1 {
		t.Fatalf("points = %d, want one point with one record", len(points))
	}
	want := attribute.NewSet(
		attribute.String("harness.host", "claude"),
		attribute.String("harness.hook", "pre-tool-use"),
		attribute.String("harness.verdict", "allow"),
	)
	if !points[0].Attributes.Equals(&want) {
		t.Fatalf("attributes = %v, want %v", points[0].Attributes, want)
	}
	if !slices.Equal(points[0].Bounds, eventbus.DurationBuckets()) {
		t.Fatalf("bounds = %v", points[0].Bounds)
	}
}

func TestHandleHookSessionIDAttribute(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		found bool
	}{
		{"with session id", `{"session_id":"abc123","cwd":"/work","tool_name":"Bash","tool_input":{"command":"ls"}}`, "abc123", true},
		{"without session id", preToolInput, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := tracetest.NewSpanRecorder()
			tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
			meter := sdkmetric.NewMeterProvider().Meter("test")
			handler := handlehook.NewHandler([]hostdomain.Codec{hookcodec.NewClaude()}, connectTo(allowAll()), tracer, meter)

			handler.Handle(context.Background(), handlehook.Command{
				Host: shareddomain.HostClaude, Hook: hostdomain.HookPreToolUse, Input: strings.NewReader(tc.input),
			})

			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("spans = %d, want 1", len(spans))
			}
			var got string
			var found bool
			for _, kv := range spans[0].Attributes() {
				if kv.Key == "harness.session.id" {
					got, found = kv.Value.AsString(), true
				}
			}
			if found != tc.found || got != tc.want {
				t.Fatalf("harness.session.id = %q (present %v), want %q (present %v)", got, found, tc.want, tc.found)
			}
		})
	}
}

func TestHandleHookTelemetryOnError(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tracer := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder)).Tracer("test")
	reader := sdkmetric.NewManualReader()
	meter := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader)).Meter("test")
	handler := handlehook.NewHandler([]hostdomain.Codec{hookcodec.NewClaude()}, connectTo(failing()), tracer, meter)

	handler.Handle(context.Background(), handlehook.Command{
		Host: shareddomain.HostClaude, Hook: hostdomain.HookPreToolUse, Input: bytes.NewReader([]byte(preToolInput)),
	})

	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	point := collected.ScopeMetrics[0].Metrics[0].Data.(metricdata.Histogram[float64]).DataPoints[0]
	got, _ := point.Attributes.Value("harness.verdict")
	if got.AsString() != "error" {
		t.Fatalf("verdict attribute = %q, want error", got.AsString())
	}
}
