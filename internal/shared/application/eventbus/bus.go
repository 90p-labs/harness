package eventbus

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/90p-labs/harness/internal/shared/domain"
)

const (
	instrumentationName = "github.com/90p-labs/harness/internal/shared/application/eventbus"
	spanPluginCall      = "plugin.call"
	metricPluginSeconds = "harness.plugin.duration"
	metricUnitSeconds   = "s"
	verdictFailure      = "failure"
	pluginFailedPrefix  = "plugin failed: "
)

const (
	attrEvent   = attribute.Key("harness.event")
	attrPlugin  = attribute.Key("harness.plugin")
	attrVerdict = attribute.Key("harness.verdict")
)

var errUnknownVerdict = errors.New("unknown verdict")

var durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}

func DurationBuckets() []float64 { return slices.Clone(durationBuckets) }

type Bus struct {
	plugins  []domain.Plugin
	tracer   trace.Tracer
	duration metric.Float64Histogram
	detail   domain.TraceDetail
}

func (b Bus) WithTraceDetail(detail domain.TraceDetail) Bus {
	b.detail = detail
	return b
}

func NewBus(plugins []domain.Plugin, tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) Bus {
	sorted := slices.Clone(plugins)
	slices.SortStableFunc(sorted, func(a, b domain.Plugin) int { return strings.Compare(a.Name(), b.Name()) })
	duration, _ := meterProvider.Meter(instrumentationName).Float64Histogram(
		metricPluginSeconds,
		metric.WithUnit(metricUnitSeconds),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	return Bus{plugins: sorted, tracer: tracerProvider.Tracer(instrumentationName), duration: duration, detail: domain.TraceDetailCompact}
}

func (b Bus) Dispatch(ctx context.Context, envelope domain.Envelope) (domain.DispatchResult, error) {
	kind, known := domain.KindOf(envelope.Event)
	if !known {
		return domain.DispatchResult{}, fmt.Errorf("%w: %q", domain.ErrUnknownEvent, envelope.Event)
	}

	var denies, deniedBy, contexts []string
	var failures []domain.DispatchFailure
	var findings []domain.Finding
	for _, plugin := range b.plugins {
		if !slices.Contains(plugin.Subscribes(), envelope.Event) {
			continue
		}
		if matcher, ok := plugin.(domain.Matcher); ok && !matcher.Matches(envelope) {
			continue
		}
		verdict, err := b.call(ctx, plugin, envelope)
		if err != nil {
			if kind == domain.KindVeto || kind == domain.KindCollect {
				failures = append(failures, domain.DispatchFailure{Plugin: plugin.Name(), Err: err})
			}
			if kind == domain.KindVeto {
				denies = []string{pluginFailedPrefix + err.Error()}
				break
			}
			continue
		}
		if kind != domain.KindObserve && verdict.Context != "" {
			contexts = append(contexts, verdict.Context)
		}
		if kind == domain.KindCollect {
			findings = append(findings, verdict.Findings...)
		}
		if verdict.Kind == domain.VerdictDeny && (kind == domain.KindVeto || kind == domain.KindCollect) {
			denies = append(denies, plugin.Name()+": "+verdict.Reason)
			deniedBy = append(deniedBy, plugin.Name())
			if kind == domain.KindVeto {
				break
			}
		}
	}

	result := domain.DispatchResult{
		Verdict:  domain.Verdict{Kind: domain.VerdictAllow, Context: strings.Join(contexts, "\n\n"), Findings: findings},
		Failures: failures,
		DeniedBy: deniedBy,
	}
	if len(denies) > 0 {
		result.Verdict.Kind = domain.VerdictDeny
		result.Verdict.Reason = strings.Join(denies, "\n")
	}
	return result, nil
}

func (b Bus) call(ctx context.Context, plugin domain.Plugin, envelope domain.Envelope) (domain.Verdict, error) {
	full := b.detail == domain.TraceDetailFull
	var span trace.Span
	if full {
		ctx, span = b.tracer.Start(ctx, spanPluginCall)
	}

	start := time.Now()
	verdict, err := plugin.Handle(ctx, envelope)
	end := time.Now()
	if err == nil && !verdict.Kind.Known() {
		err = fmt.Errorf("%w: %s: %w %q", domain.ErrPluginFailed, plugin.Name(), errUnknownVerdict, verdict.Kind)
	}

	if span == nil && (err != nil || verdict.Kind == domain.VerdictDeny) {
		_, span = b.tracer.Start(ctx, spanPluginCall, trace.WithTimestamp(start))
	}
	outcome := string(verdict.Kind)
	if err != nil {
		outcome = verdictFailure
	}
	attrs := []attribute.KeyValue{
		attrEvent.String(string(envelope.Event)),
		attrPlugin.String(plugin.Name()),
		attrVerdict.String(outcome),
	}
	if span != nil {
		if err != nil {
			span.RecordError(err, trace.WithTimestamp(end))
			span.SetStatus(codes.Error, err.Error())
		}
		span.SetAttributes(attrs...)
		span.End(trace.WithTimestamp(end))
	}
	b.duration.Record(ctx, end.Sub(start).Seconds(), metric.WithAttributes(attrs...))
	return verdict, err
}
