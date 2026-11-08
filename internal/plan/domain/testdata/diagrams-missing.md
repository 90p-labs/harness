# Plan: add the ping store and its screen

## Goal

The service stores each ping in a table. A screen lists the pings.

Track: full

## Required skills

- `skills/ddd-conventions/SKILL.md`

## Acceptance criteria

| # | Criterion | Proof |
|---|---|---|
| 1 | The store keeps each ping. | `go test ./internal/ping/domain -run TestStore -count=1` |
| 2 | The screen lists the pings. | `go test ./internal/ping/ui -run TestScreen -count=1` |

## Tasks

### 1. Ping store

- Side: backend
- Owned files: `internal/ping/domain/`, `internal/ping/infrastructure/`, `migrations/`
- Test first: `store_test.go`: `TestStore` fails, because the store does not exist.
- DoD: criterion 1.
- Depends on: none

### 2. Ping screen

- Side: frontend
- Owned files: `internal/ping/ui/`
- Test first: `screen_test.go`: `TestScreen` fails, because the screen does not exist.
- DoD: criterion 2.
- Depends on: task 1

## Decisions

- 2026-11-08: Table (lead): one table holds the pings.

## Tech specs

### Ping store

Code: task 1.
Criteria: 1

The store keeps each ping.

### Ping screen

Code: task 2.
Criteria: 2

The screen lists the pings. ![The list](ui/list.excalidraw)

### Final code tree

```tree
harness/
├── migrations/
│   └── 001_pings.sql                       new
└── internal/
    └── ping/
        ├── domain/
        │   ├── ping.go                     new
        │   ├── store.go                    new
        │   └── clock.go                    new
        ├── infrastructure/
        │   └── http/
        │       ├── handler.go              new
        │       └── openapi.yaml            new
        └── ui/
            └── screen.go                   new
```

### Final test tree

```tree
harness/
└── internal/
    └── ping/
        ├── domain/
        │   └── store_test.go               new
        └── ui/
            └── screen_test.go              new
```

### Commits

1. `feat: add the ping store` (task 1)
2. `feat: add the ping screen` (task 2)

### PR sequence

1. `feat/ping-store` (this run), based on `main`: the store and the screen.

## Open questions

- none
