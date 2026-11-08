# Plan: add the ping query

## Goal

The service answers a ping with the word pong.

## Acceptance criteria

| # | Criterion | Proof |
|---|---|---|
| 1 | The domain gives the word pong. | `go test ./internal/ping/domain -run TestPong -count=1` |
| 2 | The query returns the pong. | `go test ./internal/ping/application/query/pingquery -run TestPingQuery -count=1` |

## Tasks

### 1. Ping domain

- Side: backend
- Owned files: `internal/ping/domain/pong.go`, `internal/ping/domain/pong_test.go`, `internal/ping/domain/legacy.go`
- Test first: `pong_test.go`: `TestPong` fails, because `Pong` does not exist.
- DoD: criterion 1.
- Depends on: none

### 2. Ping query

- Side: backend
- Owned files: `internal/ping/application/query/pingquery/`, `internal/ping/module.go`
- Test first: `handler_test.go`: `TestPingQuery` fails, because the handler does not exist.
- DoD: criterion 2.
- Depends on: task 1

## Tech specs

### Final code tree

```tree
harness/
└── internal/
    └── ping/
        ├── module.go                      changed
        ├── domain/
        │   ├── legacy.go                  deleted
        │   └── pong.go                    new
        └── application/
            └── query/
                └── pingquery/
                    └── handler.go         new
```

### Final test tree

```tree
harness/
└── internal/
    └── ping/
        ├── domain/
        │   └── pong_test.go               new
        └── application/
            └── query/
                └── pingquery/
                    └── handler_test.go    new
```

### Commits

1. `feat: add the pong domain` (task 1)
2. `feat: add the ping query` (task 2)
3. `test: cover the ping query` (task 2)

### PR sequence

1. `feat/ping-query` (this run), based on `main`: the pong domain and the ping query.
