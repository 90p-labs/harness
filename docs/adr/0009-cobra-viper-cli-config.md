# 0009 Cobra for the CLI and viper for the config

## Status

Accepted, 2026-11-07.

## Context

The harness needs a CLI with subcommands and a config loader. Both are standard Go libraries with wide use.

## Decision

- Cobra builds the commands. Viper loads the config.
- This run adds cobra only. PR 2 adds viper with the first config reader, so no loader exists without a reader.
- Each module keeps its cobra commands in `internal/<module>/ui/cli/`. `cmd/harness/main.go` only wires the modules.
- PR 2 splits the exit codes. A usage error exits 2, and a runtime error exits 1.

## Consequences

- A module owns its commands and tests them alone.
- Scripts can tell a wrong call from a failed run by the exit code.
- The first config reader proves the viper loader.
