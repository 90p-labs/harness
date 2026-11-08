# 0002 Exec plugins with JSON on stdin and stdout

## Status

Accepted, 2026-11-07.

## Context

Kodify has no extension point. A new guard rule edits four files. The harness needs a plugin model that works in any language and in the core itself.

## Decision

- A plugin is any executable. The core sends a JSON envelope on stdin and reads a JSON verdict on stdout.
- Built-in pieces and exec plugins implement the same `Plugin` port. The bus sends each event to its subscribers in name order.
- An event has one of four kinds. A `veto` event stops at the first deny. A `collect` event calls every plugin and joins the denies. A `context` event keeps context texts. An `observe` event drops every verdict.
- The bus returns a plugin failure apart from a deny. On a `veto` event the failure counts as a deny, so the event fails closed.
- A harness error on a veto event fails closed with a `harness error:` deny.
- PR 5 checks the Codex hook fields and file path against the Codex hooks docs before it writes the codec.
- Codex has no `subagent.stopped` event. `DecideStop` then gets `agentRunning` false, and the session context says so.

## Consequences

- A new guard rule is one registration.
- A plugin can use any language, at the cost of one process for each call.
- The caller of a `collect` event decides what to do with a failure.
- The core sets a timeout on each call and returns `ErrPluginFailed` with the last 512 bytes of stderr.
