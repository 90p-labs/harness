# 0015 Prove architecture rules at setup

## Status

Accepted, 2026-11-09.

## Context

ADR 0013 and ADR 0014 make each project commit a canary folder for each rule, the manifest `lint/rules.yaml` and a `lint-canaries` gate. The user found these files too much in every project. A project needs the proof once, when the setup writes the tool config, not on each run.

A proof folder in a scratchpad outside the repository fails today. `git ls-files` exits 128 outside a repository. The wrappers find the repository with `git rev-parse` and mount it. The old manifest refused a path outside the repository. So the proof folder sits inside the work tree, untracked.

## Decision

- The `lint-setup` skill proves the rules once. The `linter` agent writes the proof folder `lint/proof/`. It holds `rules.yaml` and one canary folder for each rule.
- `rules.yaml` keeps the schema of the old manifest: `pack`, and for each rule a `canary` or a `waived` reason, or the `engines` form. A canary path is relative to the proof folder.
- `harness lint canaries --proof <folder>` reads `<folder>/rules.yaml`. The flag is required, and a missing flag exits 2 with the usage. A proof folder outside the work tree is an error. A canary path that leaves the proof folder is an error.
- The output prints the canary path as written in `rules.yaml`, such as `PASS ddd/domain-purity ddd-domain-purity`.
- On a `FAIL`, the skill stops, keeps the folder and reports. On success, the skill deletes `lint/proof/` and checks that `git status --porcelain` names no file under it.
- The skill reports each `PASS` line and each `WAIVED` line with its reason to the user, so a waiver is not silent. A waived rule id has no rule in the custom pack.
- No `.gitignore` rule may match `lint/proof/`, because the file list drops an ignored file.
- `harness gates init` writes no `lint-canaries` gate.
- `harness init` prints the `lint-setup` line for a detected language only when no active plugin rule with a catalogue id has a scope that ends with the language extension. It reads no manifest.
- With only built-in rules active, `harness lint canaries --proof <folder>` with no `rules.yaml` prints one line and exits 0.

## Consequences

- A project commits no canary, no `lint/rules.yaml` and no gate. It keeps the tool config, the wrapper plugin and the custom pack.
- Accepted cost: a later edit of the tool config that breaks a rule stays green until someone runs `lint-setup` again.
- A waiver shows only in the report of the setup. It is not in a committed file.
- A repository that still has a `lint-canaries` gate gets a usage error from `harness lint canaries`. The user removes the gate from `.harness.yaml`.
- The envelope `cwd` is still the canary folder under `harness lint canaries --proof`.
- ADR 0015 supersedes the canary, manifest and gate parts of ADR 0013, and the manifest parts of ADR 0014.
