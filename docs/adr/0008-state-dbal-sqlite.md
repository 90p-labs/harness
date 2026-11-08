# 0008 State behind a DBAL with SQLite

## Status

Accepted, 2026-11-07.

## Context

Kodify keeps the state in JSON files under a file lock, on one machine. The harness needs a state store that can grow to remote engines.

## Decision

- The state sits behind a DBAL. The default engine is embedded SQLite.
- PostgreSQL and MariaDB can follow as remote state.
- This run designs the state and records it here. PR 1 builds it.
- The run store uses optimistic versions. A SQLite database opens with WAL, a busy timeout of 5000 ms and foreign keys on.

## Consequences

- One machine needs no server.
- A remote engine adds a dialect and does not change the store code.
- An unknown DSN scheme returns `ErrUnknownEngine`.
- Migrations are numbered files, and the DBAL stores one version for each module.
