package eventbus_test

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/90p-labs/harness/internal/shared/application/eventbus"
	"github.com/90p-labs/harness/internal/shared/domain"
)

type fakePlugin struct {
	name    string
	events  []domain.EventName
	verdict domain.Verdict
	err     error
	calls   *[]string
}

func (f fakePlugin) Name() string                   { return f.name }
func (f fakePlugin) Subscribes() []domain.EventName { return f.events }
func (f fakePlugin) Handle(context.Context, domain.Envelope) (domain.Verdict, error) {
	*f.calls = append(*f.calls, f.name)
	return f.verdict, f.err
}

type fixture struct {
	calls    []string
	spans    *tracetest.SpanRecorder
	reader   *sdkmetric.ManualReader
	newBus   func(plugins ...domain.Plugin) eventbus.Bus
	dispatch func(bus eventbus.Bus, event domain.EventName) (domain.DispatchResult, error)
}

func newFixture() *fixture {
	f := &fixture{
		spans:  tracetest.NewSpanRecorder(),
		reader: sdkmetric.NewManualReader(),
	}
	tracerProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(f.spans))
	meterProvider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(f.reader))
	f.newBus = func(plugins ...domain.Plugin) eventbus.Bus {
		return eventbus.NewBus(plugins, tracerProvider, meterProvider)
	}
	f.dispatch = func(bus eventbus.Bus, event domain.EventName) (domain.DispatchResult, error) {
		return bus.Dispatch(context.Background(), domain.Envelope{Version: domain.EnvelopeVersion, Event: event})
	}
	return f
}

func (f *fixture) plugin(name string, events []domain.EventName, verdict domain.Verdict, err error) fakePlugin {
	return fakePlugin{name: name, events: events, verdict: verdict, err: err, calls: &f.calls}
}

func allow() domain.Verdict { return domain.Verdict{Kind: domain.VerdictAllow} }

func deny(reason string) domain.Verdict {
	return domain.Verdict{Kind: domain.VerdictDeny, Reason: reason}
}

func withContext(text string) domain.Verdict {
	return domain.Verdict{Kind: domain.VerdictContext, Context: text}
}

func events(names ...domain.EventName) []domain.EventName { return names }

func TestDispatchVeto(t *testing.T) {
	boom := errors.New("boom")
	tool := events(domain.EventToolBefore)

	t.Run("first deny stops the dispatch and keeps earlier context", func(t *testing.T) {
		f := newFixture()
		bus := f.newBus(
			f.plugin("c", tool, deny("late"), nil),
			f.plugin("a", tool, withContext("ctx-a"), nil),
			f.plugin("b", tool, deny("no push"), nil),
		)

		result, err := f.dispatch(bus, domain.EventToolBefore)

		if err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		want := domain.Verdict{Kind: domain.VerdictDeny, Reason: "b: no push", Context: "ctx-a"}
		if !reflect.DeepEqual(result.Verdict, want) {
			t.Fatalf("verdict = %+v, want %+v", result.Verdict, want)
		}
		if !slices.Equal(f.calls, []string{"a", "b"}) {
			t.Fatalf("calls = %v, want [a b]", f.calls)
		}
	})

	t.Run("a plugin failure is a deny and stops the dispatch", func(t *testing.T) {
		f := newFixture()
		bus := f.newBus(
			f.plugin("a", tool, withContext("ctx-a"), nil),
			f.plugin("b", tool, domain.Verdict{}, boom),
			f.plugin("c", tool, allow(), nil),
		)

		result, err := f.dispatch(bus, domain.EventToolBefore)

		if err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		want := domain.Verdict{Kind: domain.VerdictDeny, Reason: "plugin failed: boom", Context: "ctx-a"}
		if !reflect.DeepEqual(result.Verdict, want) {
			t.Fatalf("verdict = %+v, want %+v", result.Verdict, want)
		}
		if len(result.Failures) != 1 || result.Failures[0].Plugin != "b" || !errors.Is(result.Failures[0].Err, boom) {
			t.Fatalf("failures = %+v", result.Failures)
		}
		if !slices.Equal(f.calls, []string{"a", "b"}) {
			t.Fatalf("calls = %v, want [a b]", f.calls)
		}
	})

	t.Run("an unknown verdict kind is a failure", func(t *testing.T) {
		f := newFixture()
		bus := f.newBus(f.plugin("a", tool, domain.Verdict{Kind: "block", Reason: "no push"}, nil))

		result, err := f.dispatch(bus, domain.EventToolBefore)

		if err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		if result.Verdict.Kind != domain.VerdictDeny || !strings.HasPrefix(result.Verdict.Reason, "plugin failed: ") {
			t.Fatalf("verdict = %+v, want a plugin failed deny", result.Verdict)
		}
		if len(result.Failures) != 1 || !errors.Is(result.Failures[0].Err, domain.ErrPluginFailed) {
			t.Fatalf("failures = %+v", result.Failures)
		}
	})

	t.Run("no deny allows and keeps the context joined by a blank line", func(t *testing.T) {
		f := newFixture()
		bus := f.newBus(
			f.plugin("b", tool, withContext("two"), nil),
			f.plugin("a", tool, withContext("one"), nil),
		)

		result, _ := f.dispatch(bus, domain.EventToolBefore)

		want := domain.Verdict{Kind: domain.VerdictAllow, Context: "one\n\ntwo"}
		if !reflect.DeepEqual(result.Verdict, want) {
			t.Fatalf("verdict = %+v, want %+v", result.Verdict, want)
		}
	})
}

func TestDispatchRouting(t *testing.T) {
	f := newFixture()
	bus := f.newBus(
		f.plugin("zeta", events(domain.EventToolBefore), allow(), nil),
		f.plugin("other", events(domain.EventGateBefore), allow(), nil),
		f.plugin("alpha", events(domain.EventToolBefore, domain.EventGateBefore), allow(), nil),
		f.plugin("mid", events(domain.EventToolBefore), allow(), nil),
	)

	if _, err := f.dispatch(bus, domain.EventToolBefore); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if !slices.Equal(f.calls, []string{"alpha", "mid", "zeta"}) {
		t.Fatalf("calls = %v, want [alpha mid zeta]", f.calls)
	}
}

func TestDispatchCollect(t *testing.T) {
	boom := errors.New("source down")
	signals := events(domain.EventSignalsCheck)
	f := newFixture()
	bus := f.newBus(
		f.plugin("d", signals, deny("latency high"), nil),
		f.plugin("a", signals, deny("errors high"), nil),
		f.plugin("b", signals, domain.Verdict{}, boom),
		f.plugin("c", signals, withContext("note"), nil),
	)

	result, err := f.dispatch(bus, domain.EventSignalsCheck)

	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	want := domain.Verdict{Kind: domain.VerdictDeny, Reason: "a: errors high\nd: latency high", Context: "note"}
	if !reflect.DeepEqual(result.Verdict, want) {
		t.Fatalf("verdict = %+v, want %+v", result.Verdict, want)
	}
	if len(result.Failures) != 1 || result.Failures[0].Plugin != "b" || !errors.Is(result.Failures[0].Err, boom) {
		t.Fatalf("failures = %+v", result.Failures)
	}
	if !slices.Equal(f.calls, []string{"a", "b", "c", "d"}) {
		t.Fatalf("calls = %v, want [a b c d]", f.calls)
	}
}

func TestDispatchCollectFindings(t *testing.T) {
	first := domain.Finding{Rule: "team/no-todo", Severity: domain.SeverityError, File: "src/a.ts", Line: 2, Message: "A TODO is left."}
	second := domain.Finding{Rule: "team/no-fixme", Severity: domain.SeverityWarn, File: "src/b.ts", Line: 7, Message: "A FIXME is left."}
	found := func(findings ...domain.Finding) domain.Verdict {
		return domain.Verdict{Kind: domain.VerdictAllow, Findings: findings}
	}
	lint := events(domain.EventLintCheck)
	f := newFixture()
	collect := f.newBus(f.plugin("z", lint, found(second), nil), f.plugin("a", lint, found(first), nil))

	result, err := f.dispatch(collect, domain.EventLintCheck)

	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	if !slices.Equal(result.Verdict.Findings, []domain.Finding{first, second}) {
		t.Fatalf("findings = %+v, want the findings of a then z", result.Verdict.Findings)
	}

	observe := f.newBus(f.plugin("a", events(domain.EventGateAfter), found(first), nil))
	result, err = f.dispatch(observe, domain.EventGateAfter)

	if err != nil || len(result.Verdict.Findings) != 0 {
		t.Fatalf("observe dispatch = %+v, %v, want no findings", result.Verdict, err)
	}
}

func TestDispatchContextObserve(t *testing.T) {
	boom := errors.New("boom")

	t.Run("context keeps texts, drops denies and reports failures", func(t *testing.T) {
		f := newFixture()
		session := events(domain.EventSessionStarted)
		bus := f.newBus(
			f.plugin("a", session, withContext("hello"), nil),
			f.plugin("b", session, deny("ignored"), nil),
			f.plugin("c", session, domain.Verdict{}, boom),
		)

		result, err := f.dispatch(bus, domain.EventSessionStarted)

		if err != nil {
			t.Fatalf("dispatch: %v", err)
		}
		want := domain.Verdict{Kind: domain.VerdictAllow, Context: "hello"}
		if !reflect.DeepEqual(result.Verdict, want) {
			t.Fatalf("verdict = %+v, want %+v", result.Verdict, want)
		}
		if wantFailures := []domain.DispatchFailure{{Plugin: "c", Err: boom}}; !reflect.DeepEqual(result.Failures, wantFailures) {
			t.Fatalf("failures = %+v, want %+v", result.Failures, wantFailures)
		}
		if len(f.calls) != 3 {
			t.Fatalf("calls = %v, want 3", f.calls)
		}
	})

	// An observe plugin cannot block, but its error must still reach the caller, or it fails without a trace.
	t.Run("observe drops every verdict and reports failures", func(t *testing.T) {
		f := newFixture()
		stopped := events(domain.EventSubagentStopped)
		bus := f.newBus(
			f.plugin("a", stopped, withContext("hello"), nil),
			f.plugin("b", stopped, deny("ignored"), nil),
			f.plugin("c", stopped, domain.Verdict{}, boom),
		)

		result, _ := f.dispatch(bus, domain.EventSubagentStopped)

		if !reflect.DeepEqual(result.Verdict, allow()) {
			t.Fatalf("verdict = %+v, want allow", result.Verdict)
		}
		if want := []domain.DispatchFailure{{Plugin: "c", Err: boom}}; !reflect.DeepEqual(result.Failures, want) {
			t.Fatalf("failures = %+v, want %+v", result.Failures, want)
		}
		if len(f.calls) != 3 {
			t.Fatalf("calls = %v, want 3", f.calls)
		}
	})
}

func TestDispatchUnknownEvent(t *testing.T) {
	f := newFixture()
	bus := f.newBus(f.plugin("a", events("nope"), allow(), nil))

	_, err := f.dispatch(bus, "nope")

	if !errors.Is(err, domain.ErrUnknownEvent) {
		t.Fatalf("err = %v, want ErrUnknownEvent", err)
	}
	if len(f.calls) != 0 || len(f.spans.Ended()) != 0 {
		t.Fatalf("calls = %v, spans = %d", f.calls, len(f.spans.Ended()))
	}
}

func attributesOf(set []attribute.KeyValue) map[string]string {
	out := map[string]string{}
	for _, kv := range set {
		out[string(kv.Key)] = kv.Value.AsString()
	}
	return out
}

func TestDispatchSpans(t *testing.T) {
	f := newFixture()
	tool := events(domain.EventToolBefore)
	bus := f.newBus(
		f.plugin("a", tool, allow(), nil),
		f.plugin("b", tool, domain.Verdict{}, errors.New("boom")),
	).WithTraceDetail(domain.TraceDetailFull)

	if _, err := f.dispatch(bus, domain.EventToolBefore); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	ended := f.spans.Ended()
	if len(ended) != 2 {
		t.Fatalf("spans = %d, want 2", len(ended))
	}
	for _, span := range ended {
		if span.Name() != "plugin.call" {
			t.Fatalf("span name = %q", span.Name())
		}
	}
	first, second := ended[0], ended[1]
	wantFirst := map[string]string{"harness.event": "tool.before", "harness.plugin": "a", "harness.verdict": "allow"}
	if got := attributesOf(first.Attributes()); !mapsEqual(got, wantFirst) {
		t.Fatalf("first attributes = %v, want %v", got, wantFirst)
	}
	if first.Status().Code == codes.Error {
		t.Fatalf("first span status = %v", first.Status())
	}
	wantSecond := map[string]string{"harness.event": "tool.before", "harness.plugin": "b", "harness.verdict": "failure"}
	if got := attributesOf(second.Attributes()); !mapsEqual(got, wantSecond) {
		t.Fatalf("second attributes = %v, want %v", got, wantSecond)
	}
	if second.Status().Code != codes.Error {
		t.Fatalf("second span status = %v, want error", second.Status())
	}
	if len(second.Events()) != 1 || second.Events()[0].Time.After(second.EndTime()) {
		t.Fatalf("exception events = %+v, want one not after the span end %v", second.Events(), second.EndTime())
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestDispatchMetrics(t *testing.T) {
	f := newFixture()
	tool := events(domain.EventToolBefore)
	bus := f.newBus(
		f.plugin("a", tool, deny("no"), nil),
	)

	if _, err := f.dispatch(bus, domain.EventToolBefore); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	var collected metricdata.ResourceMetrics
	if err := f.reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("collect: %v", err)
	}
	var found *metricdata.Metrics
	for _, scope := range collected.ScopeMetrics {
		for i := range scope.Metrics {
			if scope.Metrics[i].Name == "harness.plugin.duration" {
				found = &scope.Metrics[i]
			}
		}
	}
	if found == nil {
		t.Fatal("harness.plugin.duration not recorded")
	}
	if found.Unit != "s" {
		t.Fatalf("unit = %q, want s", found.Unit)
	}
	histogram, ok := found.Data.(metricdata.Histogram[float64])
	if !ok || len(histogram.DataPoints) != 1 {
		t.Fatalf("data = %+v", found.Data)
	}
	point := histogram.DataPoints[0]
	if point.Count != 1 {
		t.Fatalf("count = %d, want 1", point.Count)
	}
	wantBounds := []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	if !slices.Equal(point.Bounds, wantBounds) {
		t.Fatalf("bounds = %v, want %v", point.Bounds, wantBounds)
	}
	want := map[string]string{"harness.event": "tool.before", "harness.plugin": "a", "harness.verdict": "deny"}
	if got := attributesOf(point.Attributes.ToSlice()); !mapsEqual(got, want) {
		t.Fatalf("attributes = %v, want %v", got, want)
	}
}

type filteredPlugin struct {
	fakePlugin
	match bool
}

func (p filteredPlugin) Matches(domain.Envelope) bool { return p.match }

func TestBusSkipsFilteredPlugin(t *testing.T) {
	f := newFixture()
	skipped := filteredPlugin{fakePlugin: f.plugin("skipped", events(domain.EventRunTransitionAfter), allow(), nil), match: false}
	bus := f.newBus(skipped)

	if _, err := f.dispatch(bus, domain.EventRunTransitionAfter); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if len(f.calls) != 0 || len(f.spans.Ended()) != 0 {
		t.Fatalf("calls = %v, spans = %d", f.calls, len(f.spans.Ended()))
	}
}

func TestBusPlainSubscriptionReceivesAll(t *testing.T) {
	f := newFixture()
	matched := filteredPlugin{fakePlugin: f.plugin("matched", events(domain.EventRunTransitionAfter), allow(), nil), match: true}
	bus := f.newBus(f.plugin("plain", events(domain.EventRunTransitionAfter), allow(), nil), matched)

	if _, err := f.dispatch(bus, domain.EventRunTransitionAfter); err != nil {
		t.Fatalf("dispatch: %v", err)
	}

	if !slices.Equal(f.calls, []string{"matched", "plain"}) {
		t.Fatalf("calls = %v, want [matched plain]", f.calls)
	}
}

func TestDispatchNamesDenyingPlugins(t *testing.T) {
	f := newFixture()
	tool := events(domain.EventToolBefore)
	bus := f.newBus(
		f.plugin("a", tool, allow(), nil),
		f.plugin("b", tool, deny("no"), nil),
		f.plugin("c", tool, deny("never"), nil),
	)

	result, err := f.dispatch(bus, domain.EventToolBefore)

	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"b"}; !slices.Equal(result.DeniedBy, want) {
		t.Errorf("DeniedBy = %v, want %v (a veto stops at the first deny)", result.DeniedBy, want)
	}
}

func TestPluginCallSpanDetail(t *testing.T) {
	boom := errors.New("boom")
	tool := events(domain.EventToolBefore)
	for _, tc := range []struct {
		name         string
		verdict      domain.Verdict
		err          error
		compactSpans int
	}{
		{"allow", allow(), nil, 0},
		{"deny", deny("no"), nil, 1},
		{"error", domain.Verdict{}, boom, 1},
	} {
		for _, detail := range []domain.TraceDetail{domain.TraceDetailCompact, domain.TraceDetailFull} {
			t.Run(tc.name+"/"+string(detail), func(t *testing.T) {
				f := newFixture()
				bus := f.newBus(f.plugin("p", tool, tc.verdict, tc.err)).WithTraceDetail(detail)

				_, _ = f.dispatch(bus, domain.EventToolBefore)

				want := tc.compactSpans
				if detail == domain.TraceDetailFull {
					want = 1
				}
				if got := len(f.spans.Ended()); got != want {
					t.Errorf("spans = %d, want %d", got, want)
				}
				if got := histogramCount(t, f.reader); got != 1 {
					t.Errorf("duration samples = %d, want 1", got)
				}
			})
		}
	}
}

func histogramCount(t *testing.T, reader sdkmetric.Reader) uint64 {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatal(err)
	}
	var count uint64
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			if h, ok := m.Data.(metricdata.Histogram[float64]); ok {
				for _, p := range h.DataPoints {
					count += p.Count
				}
			}
		}
	}
	return count
}
