# 0012 Deploy tools as plugins on the event bus

## Status

Accepted, 2026-11-08.

## Context

The harness calls two deploy tools in fixed code. `harness gates init` proposes `helm-render` and `promtool-test` only when `lint.packs` has `observability`. The harness lint engine runs `helm template` itself. A project with another deploy tool, such as kubeconform or `terraform validate`, cannot add its gates. ADR 0002 gives one `Plugin` port for built-in pieces and exec plugins.

## Decision

- A deploy tool is a plugin on two `collect` events, named in the `<noun>.<verb>` style of `signals.check`.
- `gates.detect` sends the repo files. Each plugin answers with its proposed gates, one JSON gate on each line of `Verdict.Context`. `gates init` appends them to the toolchain gates.
- `chart.render` sends a chart folder. The plugin answers with the rendered manifest in `Verdict.Context`, in the `helm template` format with a `# Source: <chart name>/<file>` comment on each document. The harness lint engine calls it for the chart rules.
- `helm` and `promtool` are default in-process plugins in the new `deploy` module. They need no config and propose nothing without their files.
- A project adds a deploy tool with a `plugins:` entry on `gates.detect`. It turns a default plugin off with `deploy.disabled`.
- The data travels in `Verdict.Context`, so the `Verdict` type and the bus do not change.

## Consequences

- The deploy gates follow the files: a `Chart.yaml` gives `helm-render`, a `*.rules.test.yaml` gives `promtool-test`. They no longer need `lint.packs: [observability]`.
- A plugin failure, a deny, a bad line or a duplicate gate id stops `gates init` before it writes.
- Only one plugin renders a chart. An exec plugin on `chart.render` needs `helm` in `deploy.disabled`.
- With no renderer, or with a document that has no `# Source:` comment, the chart rules stop with an error, so a missing render cannot pass as clean.
- The live flow does not change. It calls no deploy tool.
