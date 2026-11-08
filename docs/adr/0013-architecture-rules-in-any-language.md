# 0013 Architecture rules in any language

## Status

Accepted, 2026-11-08.

## Context

The built-in `ddd` pack checks the layers and the bounded contexts of a TypeScript project. A PHP, Python or Go project has its own dependency rule tools, such as deptrac, import-linter and go-arch-lint. ADR 0011 lets such a tool run as a lint plugin, but each project must write its config by hand. Nothing proves later that the config still catches each rule: a wrong path in the tool config gives a clean lint, and a clean lint looks the same as a correct one.

## Decision

- The harness keeps a rule catalogue as data, embedded in the binary. Each rule has an id and a one-line description. The ids are the ids of the built-in `ddd` pack. `harness lint catalogue` prints it.
- The `lint-setup` skill and the `linter` agent set the rules up on a project. The agent writes the native tool config, a wrapper plugin on `lint.check` that maps the tool output to the catalogue ids, a custom pack with these ids, one canary for each rule and the manifest `lint/rules.yaml`. The skill asks the user each open choice, because a subagent cannot ask.
- A canary is a small committed folder that breaks one rule on purpose. `harness lint canaries` sends `lint.check` with the canary folder as the envelope `cwd` and the one rule. It fails when no finding has the rule id and a file inside the canary folder. A finding for a file outside the canary does not count, so a tool that reads the project instead of the canary cannot pass. No agent runs in this check, so it can be a gate.
- Each catalogue rule of the pack is in the manifest with a canary or with `waived: <reason>`. A missing rule fails the check, and a waived rule shows its reason, so no gap is silent. A reason needs 3 words or more, and a manifest that waives each rule it lists is an error, because it proves nothing.
- The catalogue has the coverage rule `ddd/src-roots`. The wrapper maps each file and each dependency that the tool cannot place to a finding: an unplaced file to `ddd/src-roots`, and an unplaced dependency to the rule of the dependant layer. A tool config that drops a folder then fails a canary, not only a review.
- A custom pack can use the rule ids of a built-in pack that is not in `lint.packs`.
- The built-in `ddd` pack gets no canaries. Its embedded fixtures prove each rule in the binary tests. With only built-in rules and no manifest, `harness lint canaries` prints one line and passes.
- The harness does not translate one rule model into each tool config. The tools differ too much: deptrac has layers and a ruleset, import-linter has contracts, go-arch-lint has components. A translator would need a model as wide as all the tools, and it would hide the config that the user tunes. The agent writes the native config once, the user owns it, and the canaries prove it.

## Consequences

- `harness init` prints a next step to the `lint-setup` skill for each detected language with no built-in pack: PHP, Python or Go. With `lint/rules.yaml`, it prints the step only for a language that no plugin rule with a catalogue id covers, and the step names the one-manifest limit.
- `harness gates init` adds the `lint-canaries` gate when `lint/rules.yaml` exists.
- A config edit that stops the tool from catching a rule fails the next canary check.
- A repository has one manifest for one catalogue pack, and a rule id belongs to one active pack. A clash names both packs. So a repository with two languages that both need the catalogue rules, or with the built-in `ddd` pack and another language, cannot prove both yet.
- A canary proves the rule on the canary, not on each file of the project. `ddd/src-roots` closes most of this gap: a file or a dependency that the tool cannot place is a finding. A pack scope that misses a source folder still passes the canary.
- A waiver shows only in `lint/rules.yaml` and in the canary output. `harness verify` prints the gate status, not the waivers.
