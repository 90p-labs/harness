# Contributing

This page holds the repository layout, the test commands and the bug report for the harness itself, and the [README](../README.md) gives the usage.

## Report a harness bug

An agent files a bug about the harness with the `report` skill (`/report` on Claude Code). The skill reproduces the bug on a clean repository, fills `skills/report/references/issue-template.md` and shows a dry-run to the user. It files the issue only after the user says yes:

```console
$ harness report --dry-run --title "hook: a bad Stop payload exits 2" --body-file draft.md
$ harness report --title "hook: a bad Stop payload exits 2" --body-file draft.md
https://github.com/90p-labs/harness/issues/31
```

The command refuses a draft with a missing section, adds the harness version, the OS and the active run, and redacts emails, home paths, tokens and keys.

## Repository layout and tests

Tests live beside the code or assets they check. The root keeps the module files, project documents and configuration.

| Folder | Contents and test ownership |
|---|---|
| `cmd/harness/` | Binary entry point, command wiring and CLI tests |
| `internal/` | Go modules with their layers and package tests; architecture and repository policy checks live directly here |
| `internal/testsupport/repository/` | Shared repository asset access for tests |
| `skills/` | Shared skill checks, with each skill's tests inside its own folder |
| `agents/` | Agent frontmatter and report contract tests |
| `bin/` | Plugin launcher and its shell behavior tests |
| `hooks/` | Host hook configurations, checked by the host codec tests |
| `examples/` | Example configuration checks, with Python plugin tests in `examples/python/` |
| `docs/` | Documentation consistency checks; ADR checks live in `docs/adr/` |
| `.claude-plugin/`, `.codex-plugin/`, `.agents/plugins/` | Plugin manifests and marketplaces, checked by host installation tests |

From the repository root, run `make check` for formatting, vet and all tests, or `go test ./...` for all tests alone.
To focus on one area, run `go test ./skills/...`, `go test ./docs/...` or `go test ./bin`.
Tests also run from their own folders, for example `cd skills/ddd && go test`.
