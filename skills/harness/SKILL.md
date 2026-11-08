---
name: harness
description: "Drives one change through the harness loop: plan, approval, test-first work, verify gates, review and ship. Use when the user types /harness:harness (Codex: $harness:harness) with a prompt, or with status, approve or abandon, in a repository that has .harness.yaml."
argument-hint: "[prompt | status | approve | abandon]"
---

# Harness loop

The `harness` binary holds the run state and the gates. The plugin hooks run its guard on each tool call and each stop.

Arguments: `$ARGUMENTS`

## Binary

- In each command below, `harness` means the full path `"${CLAUDE_PLUGIN_ROOT}/bin/harness"`. Write the full path in each command: a shell variable does not stay from one Bash call to the next.
- On Codex, `${CLAUDE_PLUGIN_ROOT}` shows unexpanded. Use `bin/harness` two folders above this `SKILL.md`.
- Run `harness version` first. The first call downloads or builds the binary. If it fails, stop and report the error.
- The repository needs `.harness.yaml` at its top level. Without it, the hooks do nothing: tell the user to set the repository up (README, section "Set up a repository").

## Route the arguments

- `status` or empty: run `harness run status` and continue the open run.
- `approve`: run `harness run approve` only when the user approved the plan in this conversation.
- `abandon`: run `harness run abandon` and report.
- Any other text with no active run: run `harness run start "<text>"`.

Each `run` command prints the phase and a `next:` line. Do the `next:` step.

## Phases

1. `planning`: write `plan.md` in the run folder. `harness plan --help` gives the format. Write each diagram through `harness diagram`. Run `harness plan check`, then `harness run plan`.
2. `awaiting_approval`: show the plan (`harness plan view` gives the page) and ask the user to approve it. Stop until the user answers.
3. `working`: for each task, write the failing test first, then the code. Run `harness verify` until each gate and each criterion passes. Commit, run `harness verify --commits <default branch>`, then `harness run review`.
4. `reviewing`: start the `reviewer` agent on the diff, on the model of the `reviewer` role in `.harness.yaml`. With findings, run `harness run rework --findings "<summary>"` and go back to `working`. With none, run `harness run ship`.
5. `shipping`: push and open the pull request as a draft. The user merges it. Then run `harness run done --pr <url>`.

A change to the approved plan needs `harness run amend` and a new approval.

## Guard

The guard denies some calls, for example a merge, a forced push, a read of a secret or a push before `verify --commits`. Read the deny reason and do the step that it names. Do not work around a deny with another command.
