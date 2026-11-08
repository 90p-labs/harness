# 0010 The plan view is one self-contained HTML page

## Status

Accepted, 2026-11-08.

## Context

A human approves a plan in `awaiting_approval`. A long `plan.md` hides the diagrams, the trace and the problems. The page must open from a file, in an offline review, with no server and no network.

## Decision

- `harness plan view` writes one HTML page, `plan.html`, beside the plan. It works in every phase and with no run.
- The page embeds mermaid, rough.js, the fonts and the CSS with `go:embed`. No `src`, `href` or `url(` points outside the page.
- The Go renderer draws each Excalidraw sketch to SVG through goja and rough.js. The page loads no Excalidraw library.
- The mermaid `stateDiagram-v2` marks go through a flowchart conversion, so each state and edge gets the color of its mark.
- A view plugin on `run.transition.after` writes the page when the new phase is `awaiting_approval`. A render error never blocks the transition.
- The plan rule `diagram-required` refuses a plan with a missing diagram or sketch file.

## Consequences

- The reviewer opens one file and needs no tool.
- Each page weighs about 4 MB, because it holds the mermaid bundle and the fonts.
- The binary grows by the embedded assets and the goja dependency.
- A page shows `not run` for a proof or a branch that it cannot read, and lists the read error as a problem.
