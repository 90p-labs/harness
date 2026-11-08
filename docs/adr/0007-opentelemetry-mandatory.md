# 0007 OpenTelemetry is mandatory

## Status

Accepted, 2026-11-07.

## Context

Kodify builds one tracer provider for each span, has no metrics and gives the modules no telemetry API. The harness must be observable from the first release.

## Decision

- The harness ships `opentelemetry-go` by default. Traces and metrics are always on.
- `Setup` installs one tracer provider and one meter provider as the global providers.
- The harness exports with OTLP when the standard `OTEL_EXPORTER_OTLP_ENDPOINT` env is set. Without it, no data leaves the machine.
- The exporters have retry off. The shutdown flushes both providers under one limit of 2 seconds.
- The histogram sets its bucket bounds through the API, with no view.
- A telemetry error prints one `telemetry:` line on stderr and does not change the exit code.

## Consequences

- Application code uses the telemetry API and imports no SDK. A test checks it.
- A user sees no network traffic from telemetry unless the user sets an endpoint.
- A dead collector delays the exit by at most 2 seconds.
