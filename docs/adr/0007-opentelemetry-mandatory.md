# 0007 OpenTelemetry is mandatory

## Status

Accepted, 2026-11-07.

## Context

Kodify builds one tracer provider for each span, has no metrics and gives the modules no telemetry API. The harness must be observable from the first release.

## Decision

- The harness ships `opentelemetry-go` by default. Traces and metrics are always on.
- `Setup` installs one tracer provider and one meter provider as the global providers.
- The harness exports with OTLP when the standard `OTEL_EXPORTER_OTLP_ENDPOINT` env or its `HARNESS_` alias, `HARNESS_OTEL_EXPORTER_OTLP_ENDPOINT`, is set. Without one, no data leaves the machine.
- Claude Code removes every `OTEL_*` variable from the env of its hooks and tools. The `HARNESS_` alias wins over the standard name, because Claude Code reads the `OTEL_` names for its own telemetry.
- The exporters have retry off. The shutdown flushes both providers under one limit of 2 seconds.
- The histogram sets its bucket bounds through the API, with no view.
- A telemetry error prints one `telemetry:` line on stderr and does not change the exit code.
- A gate child process gets `TRACEPARENT`. A command reads it at start as the parent of its spans.
- Each run has one trace. The harness derives the trace ID and the ID of the root span `run` from the run ID.
- A process of an active run is a child of the root span. A `TRACEPARENT` in the same trace stays the parent. A parent in another trace is replaced by the root span.
- The harness exports the root span when the run reaches done or abandoned, for example through `harness run done`, `harness run abandon` or the live check.
- The harness exports an `agent.turn` span for each agent turn and an `agent.subagent` span for each subagent. It builds them from the hook events. The start time is stored in the state database.

## Consequences

- Application code uses the telemetry API and imports no SDK. A test checks it.
- The verify run is one trace tree. The `lint.run` span sits under `gate.run`, and each `lint.engine` span under `lint.run`.
- The whole run is one trace tree under the root span `run`. The root span appears only after the run reaches done or abandoned. Until then, or if that export fails, the trace has no root span.
- A gap between hook spans, for example model time or a server tool call, sits inside an `agent.turn` or `agent.subagent` span.
- A user sees no network traffic from telemetry unless the user sets an endpoint.
- A dead collector delays the exit by at most 2 seconds.
