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

	"github.com/jorge07/harness/internal/shared/domain"
)

const (
	instrumentationName = "github.com/jorge07/harness/internal/shared/application/eventbus"
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

type Bus struct {
	plugins  []domain.Plugin
	tracer   trace.Tracer
	duration metric.Float64Histogram
}

func NewBus(plugins []domain.Plugin, tracerProvider trace.TracerProvider, meterProvider metric.MeterProvider) Bus {
	sorted := slices.Clone(plugins)
	slices.SortStableFunc(sorted, func(a, b domain.Plugin) int { return strings.Compare(a.Name(), b.Name()) })
	duration, _ := meterProvider.Meter(instrumentationName).Float64Histogram(
		metricPluginSeconds,
		metric.WithUnit(metricUnitSeconds),
		metric.WithExplicitBucketBoundaries(durationBuckets...),
	)
	return Bus{plugins: sorted, tracer: tracerProvider.Tracer(instrumentationName), duration: duration}
}

func (b Bus) Dispatch(ctx context.Context, envelope domain.Envelope) (domain.DispatchResult, error) {
	kind, known := domain.KindOf(envelope.Event)
	if !known {
		return domain.DispatchResult{}, fmt.Errorf("%w: %q", domain.ErrUnknownEvent, envelope.Event)
	}

	var denies, contexts []string
	var failures []domain.DispatchFailure
	for _, plugin := range b.plugins {
		if !slices.Contains(plugin.Subscribes(), envelope.Event) {
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
		if verdict.Kind == domain.VerdictDeny && (kind == domain.KindVeto || kind == domain.KindCollect) {
			denies = append(denies, plugin.Name()+": "+verdict.Reason)
			if kind == domain.KindVeto {
				break
			}
		}
	}

	result := domain.DispatchResult{
		Verdict:  domain.Verdict{Kind: domain.VerdictAllow, Context: strings.Join(contexts, "\n\n")},
		Failures: failures,
	}
	if len(denies) > 0 {
		result.Verdict.Kind = domain.VerdictDeny
		result.Verdict.Reason = strings.Join(denies, "\n")
	}
	return result, nil
}

func (b Bus) call(ctx context.Context, plugin domain.Plugin, envelope domain.Envelope) (domain.Verdict, error) {
	ctx, span := b.tracer.Start(ctx, spanPluginCall)
	defer span.End()

	start := time.Now()
	verdict, err := plugin.Handle(ctx, envelope)
	if err == nil && !verdict.Kind.Known() {
		err = fmt.Errorf("%w: %s: %w %q", domain.ErrPluginFailed, plugin.Name(), errUnknownVerdict, verdict.Kind)
	}

	outcome := string(verdict.Kind)
	if err != nil {
		outcome = verdictFailure
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	attrs := []attribute.KeyValue{
		attrEvent.String(string(envelope.Event)),
		attrPlugin.String(plugin.Name()),
		attrVerdict.String(outcome),
	}
	span.SetAttributes(attrs...)
	b.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	return verdict, err
}
