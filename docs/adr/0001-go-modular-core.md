# 0001 Go modular core

## Status

Accepted, 2026-11-07.

## Context

Kodify is a TypeScript harness with company parts and several known defects. The project needs an open source harness that anyone can extend.

The module is `github.com/jorge07/harness`. The binary is `harness`. The license is MIT.

## Decision

- Write the harness in Go, as new code from zero. Do not fork kodify. Do not keep a TypeScript core.
- Build the core from independent modules. Each module has its own layers.
- Use the own CQRS layout: `application/command/<name>/` and `application/query/<name>/`. The event bus is not a use case.
- Move `Host` to `internal/shared/domain/`, because the envelope and the config both use it.
- Add `At` to `Command`, so the domain reads no clock. `Apply` records a transition event.

## Consequences

- A module can change with no edit in the other modules.
- A layer check in a test keeps domain code free of infrastructure imports.
- The harness has no company parts. The users add them as plugins.
- The project has to rebuild the kodify features that it needs, in the roadmap order.
