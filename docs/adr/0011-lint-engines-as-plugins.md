# 0011 Lint engines are plugins on the event bus

## Status

Accepted, 2026-11-08.

## Context

The binary fixes the lint engines: `harness`, `biome-grit`, `biome-rule` and `oxlint`. A custom pack can use only the Biome and oxlint engines. A team that has its own linter, such as a script or another tool, cannot add it to the lint report, the baseline ratchet or the gates. ADR 0002 already gives one extension point: built-in pieces and exec plugins implement the same `Plugin` port on the event bus.

## Decision

- A lint engine is a plugin on the event bus. Two new `collect` events carry the contract: `lint.check` for `harness lint` and `lint.prepare` for `harness lint init`.
- The payload holds the rules of that engine. On `lint.check` it also holds the git files that match the rule scopes. The verdict returns the findings in a new `findings` field.
- The rule `engine` is the plugin name. The lint module builds one bus for each engine name, so each plugin gets only its own rules.
- The three built-in engine adapters are default plugins in process: `harness`, `biome-grit` (it also runs the `biome-rule` rules) and `oxlint`. They are on with no config.
- A project adds a lint plugin with the `plugins` key: a name, a command and the event `lint.check`, and `lint.prepare` when it writes config files. A custom pack rule names the plugin as its engine.
- `lint.disabled` turns a default lint plugin off. Its name is then free for an exec plugin.
- The lint module refuses a finding whose rule is not one of the sent rules, or whose file is absolute, because the baseline key holds both. A finding takes the severity of its rule, so the pack, not the plugin, decides the gate exit code. A pack rule with an unknown severity is an error when the pack loads.
- Rejected: Go plugin packages (`plugin.Open`). They work only on Linux and macOS, need the same Go version, module versions and build flags as the binary, and cannot be unloaded. A team would have to build its linter against each harness release. An exec plugin works in any language, with the JSON contract of ADR 0002.

## Consequences

- A team linter gives findings that go through the same report, baseline ratchet and gates as the built-in rules.
- An exec lint plugin costs one process for each `harness lint` run and engine. Its default timeout is 5 seconds; a slow linter sets `timeout` in its `plugins` entry.
- A rule engine with no plugin, an exec plugin with the name of an enabled default or the name `biome-rule`, and an exec plugin on `lint.prepare` without `lint.check` are config errors before any run.
- The `lint.engine` span goes: the bus `plugin.call` span with `harness.plugin` carries the engine name.
