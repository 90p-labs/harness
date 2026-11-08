# Pack observability

This pack checks that the app and its chart are observable.

- The app starts the OpenTelemetry SDK, and the bus has an instrumented middleware.
- The rendered chart has readiness and liveness probes, immutable image tags and a valid HPA.
- Each alert has a runbook and a test. Each SLO series has an alert.
- Each dashboard query uses a recorded series.
- The rendered rules need `helm` on the path.
- Set `lint.observability.chart`, `rules` and `dashboards` for the paths.
