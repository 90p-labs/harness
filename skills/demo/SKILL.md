---
name: demo
description: "Run a visible, paced browser walkthrough of an app feature with observed outcomes and console/network diagnostics. Use for a watched demo or QA walkthrough, including /90p:demo (Codex: $90p:demo)."
---

## Decision tree

```text
Watched walkthrough → prepare the stack below, then drive the feature
Repeat a demo       → reset only its owned records before running again
Stopped run         → inspect the failed step and diagnostics before attributing a bug
Durable test        → use the project's test workflow instead
```

## Prepare and show

1. Use the project's documented local/test stack. Read [stack.md](references/stack.md) for seed ownership, browser attachment and cleanup. Reconcile the database and external-service fakes; never reset shared or production data.
2. Prefer an available supported browser tool that exposes a visible window the user can watch and take over. If it cannot provide that session, use the dedicated loopback CDP fallback in the stack guide and adapt [demo-template.ts](references/demo-template.ts). Reuse the project's Playwright installation; this plugin adds no JavaScript dependency.
3. State the feature and starting state. Handle authentication using the app's own flow, without printing credentials. Install diagnostics before navigation and keep them active through failed steps and cleanup.
4. Drive user-facing locators with bounded action/navigation timeouts. Assert each outcome, then caption what the page actually shows, using observed values. Pace captions from one delay setting; type visibly when it helps the viewer. Readability pauses do not replace readiness checks.
5. Add extra actors, mobile widths or native dialogs only when the scenario needs them. Give extra actors isolated sessions, bring the active page forward, and restore viewports afterward. Register a dialog handler before its triggering action, record a redacted message and assert the resulting state; await handler errors too.

## Report every run

Report completed steps, observed results and the last attempted step on failure. Include console warnings/errors, page errors, failed requests and HTTP 4xx/5xx responses, including on otherwise successful runs. Keep expected noise visible with its concrete cause; an unexplained error remains a finding.

Redact URL credentials, query strings, fragments and known secrets from captions, diagnostics and errors; review output before sharing. Never log headers, cookies, request bodies or authentication values. An early failure or partial observation cannot justify a clean report. Distinguish an app defect, inconsistent seed and unknown cause using evidence; fix owned demo data before rerunning.

Finish with what remains open (redacted URL and actor role), diagnostic limits, and how to stop processes started for this demo. Preserve unrelated tabs, sessions and data; leave the watched page usable for handoff.
