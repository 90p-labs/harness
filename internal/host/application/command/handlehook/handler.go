package handlehook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	hostdomain "github.com/jorge07/harness/internal/host/domain"
	"github.com/jorge07/harness/internal/shared/application/eventbus"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	spanName           = "hook.handle"
	metricHookSeconds  = "harness.hook.duration"
	metricUnitSeconds  = "s"
	outcomeError       = "error"
	maxInputBytes      = 16 << 20
	harnessErrorPrefix = "harness error: "
	missingEventLine   = "%s: this host does not send it"
	contextSeparator   = "\n\n"
	fallbackLineBreak  = "\n"
	errInputAboveLimit = "input above 16 MiB"
	errNoCodecForHost  = "no codec for host"
	attrHost           = attribute.Key("harness.host")
	attrHook           = attribute.Key("harness.hook")
	attrVerdict        = attribute.Key("harness.verdict")
	attrSessionID      = attribute.Key("harness.session.id")
)

type Connect func(ctx context.Context) (shareddomain.Dispatcher, error)

type Command struct {
	Host  shareddomain.Host
	Hook  hostdomain.HookName
	Input io.Reader
}

type Output struct {
	Stdout []byte
	Stderr string
}

type Handler struct {
	codecs   []hostdomain.Codec
	connect  Connect
	tracer   trace.Tracer
	duration metric.Float64Histogram
}

func NewHandler(codecs []hostdomain.Codec, connect Connect, tracer trace.Tracer, meter metric.Meter) Handler {
	duration, _ := meter.Float64Histogram(
		metricHookSeconds,
		metric.WithUnit(metricUnitSeconds),
		metric.WithExplicitBucketBoundaries(eventbus.DurationBuckets()...),
	)
	return Handler{codecs: codecs, connect: connect, tracer: tracer, duration: duration}
}

func (h Handler) Handle(ctx context.Context, c Command) Output {
	ctx, span := h.tracer.Start(ctx, spanName)
	defer span.End()
	start := time.Now()

	out, verdict, raw, err := h.process(ctx, c)
	outcome := string(verdict.Kind)
	if err != nil {
		outcome = outcomeError
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		out = h.failClosed(c, isVeto(c.Hook) && !isRetriedStop(c.Hook, raw), err)
	}
	attrs := []attribute.KeyValue{
		attrHost.String(string(c.Host)),
		attrHook.String(string(c.Hook)),
		attrVerdict.String(outcome),
	}
	span.SetAttributes(attrs...)
	h.duration.Record(ctx, time.Since(start).Seconds(), metric.WithAttributes(attrs...))
	return out
}

func (h Handler) process(ctx context.Context, c Command) (Output, shareddomain.Verdict, []byte, error) {
	codec, err := h.codecOf(c.Host)
	if err != nil {
		return Output{}, shareddomain.Verdict{}, nil, err
	}
	raw, err := readLimited(c.Input)
	if err != nil {
		return Output{}, shareddomain.Verdict{}, nil, err
	}
	envelope, err := codec.Decode(c.Hook, raw)
	if err != nil {
		return Output{}, shareddomain.Verdict{}, raw, err
	}
	if envelope.SessionID != "" {
		trace.SpanFromContext(ctx).SetAttributes(attrSessionID.String(envelope.SessionID))
	}
	missing := hostdomain.MissingEvents(codec)
	if envelope.Event == shareddomain.EventSessionStarted && len(missing) > 0 {
		if envelope.Payload, err = withMissingEvents(envelope.Payload, missing); err != nil {
			return Output{}, shareddomain.Verdict{}, raw, err
		}
	}
	dispatcher, err := h.connect(ctx)
	if err != nil {
		return Output{}, shareddomain.Verdict{}, raw, err
	}
	result, err := dispatcher.Dispatch(ctx, envelope)
	if err != nil {
		return Output{}, shareddomain.Verdict{}, raw, err
	}
	verdict := result.Verdict
	var stderr strings.Builder
	if len(result.Failures) > 0 && isRetriedStop(c.Hook, raw) {
		verdict = shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
		for _, failure := range result.Failures {
			stderr.WriteString(harnessErrorPrefix + failure.Plugin + ": " + singleLine(failure.Err) + "\n")
		}
	}
	if envelope.Event == shareddomain.EventSessionStarted {
		verdict.Context = appendFallbackLines(verdict.Context, missing)
	}
	stdout, err := codec.Encode(c.Hook, verdict)
	if err != nil {
		return Output{}, shareddomain.Verdict{}, raw, err
	}
	return Output{Stdout: stdout, Stderr: stderr.String()}, verdict, raw, nil
}

func (h Handler) failClosed(c Command, block bool, cause error) Output {
	codec, err := h.codecOf(c.Host)
	if err != nil {
		return Output{Stderr: harnessErrorPrefix + errors.Join(cause, err).Error() + "\n"}
	}
	stderr := harnessErrorPrefix + singleLine(cause) + "\n"
	verdict := shareddomain.Verdict{Kind: shareddomain.VerdictAllow}
	if block {
		verdict = shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: harnessErrorPrefix + singleLine(cause)}
		stderr = ""
	}
	stdout, err := codec.Encode(c.Hook, verdict)
	if err != nil {
		return Output{Stderr: harnessErrorPrefix + singleLine(errors.Join(cause, err)) + "\n"}
	}
	return Output{Stdout: stdout, Stderr: stderr}
}

func (h Handler) codecOf(host shareddomain.Host) (hostdomain.Codec, error) {
	for _, codec := range h.codecs {
		if codec.Host() == host {
			return codec, nil
		}
	}
	return nil, fmt.Errorf("%s: %q", errNoCodecForHost, host)
}

func isVeto(hook hostdomain.HookName) bool {
	event, _ := hostdomain.EventOf(hook)
	kind, _ := shareddomain.KindOf(event)
	return kind == shareddomain.KindVeto
}

// The host sends a blocked stop again with stop_hook_active, so a second block loops forever.
// It reads only that field of the raw input, because a stop that fails to decode must end the loop too.
func isRetriedStop(hook hostdomain.HookName, raw []byte) bool {
	if hook != hostdomain.HookStop {
		return false
	}
	var stop struct {
		StopHookActive bool `json:"stop_hook_active"`
	}
	return json.Unmarshal(raw, &stop) == nil && stop.StopHookActive
}

func readLimited(in io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(in, maxInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxInputBytes {
		return nil, errors.New(errInputAboveLimit)
	}
	return raw, nil
}

func withMissingEvents(payload json.RawMessage, missing []shareddomain.EventName) (json.RawMessage, error) {
	var session shareddomain.SessionPayload
	if err := json.Unmarshal(payload, &session); err != nil {
		return nil, err
	}
	session.MissingEvents = missing
	return json.Marshal(session)
}

func appendFallbackLines(existing string, missing []shareddomain.EventName) string {
	if len(missing) == 0 {
		return existing
	}
	lines := make([]string, 0, len(missing))
	for _, event := range missing {
		lines = append(lines, fmt.Sprintf(missingEventLine, event))
	}
	joined := strings.Join(lines, fallbackLineBreak)
	if existing == "" {
		return joined
	}
	return existing + contextSeparator + joined
}

func singleLine(err error) string {
	return strings.ReplaceAll(err.Error(), "\n", "; ")
}
