# 0004 Biome as the lint engine

## Status

Accepted, 2026-11-07.

## Context

Kodify keeps the DDD lint inside the guard context. The lint packs need an engine that checks code and supports custom rules. The binary must not need Node for its own checks.

## Decision

- Biome is the lint engine of the packs. GritQL plugins hold the custom rules.
- Oxlint is a fallback. Use it only for a rule that Biome cannot express.
- Go code in the binary checks folders and imports between files.
- `lint` is its own module, and the guard only calls it.

## Consequences

- A pack is data that the binary reads, with Biome rules next to it.
- A project that runs the TypeScript packs needs Biome installed.
- The folder and import checks run with no external tool.
- A rule that Biome cannot express needs an oxlint config in the pack.
