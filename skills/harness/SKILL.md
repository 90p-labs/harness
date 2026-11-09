---
name: harness
description: "Drives one change through the harness loop: plan, approval, test-first work, verify gates, review and ship. Use when the user types /harness:harness (Codex: $harness:harness) with a prompt, or with status, approve or abandon, in a repository that has .harness.yaml."
argument-hint: "[prompt | status | approve | abandon]"
---

# Harness loop

The `harness` binary holds the run state and the gates. The plugin hooks run its guard on each tool call and each stop.

Arguments: `$ARGUMENTS`

## Decision tree

```
Start: run harness version ("Binary")
├── the argument → "Route the arguments", then the phase that it prints in "Phases"
├── harness init prints a lint-setup line → the `lint-setup` skill, one language at a time
├── the change touches a bounded context of a DDD project
│   └── load the `ddd` skill, and give its path to the explorer and the reviewer
├── the phase shipping → the `pr-open` skill
└── a deny of the guard → "Guard"
```

## Binary

- In each command below, `harness` means the full path `"${CLAUDE_PLUGIN_ROOT}/bin/harness"`. Write the full path in each command: a shell variable does not stay from one Bash call to the next.
- On Codex, `${CLAUDE_PLUGIN_ROOT}` shows unexpanded. Use `bin/harness` two folders above this `SKILL.md`.
- Run `harness version` first. The first call downloads or builds the binary. If it fails, stop and report the error.
- The repository needs `.harness.yaml` at its top level. Without it, the hooks do nothing: tell the user to set the repository up (README, section "Set up a repository").
- After `harness init`, read its output. For each line `next: run the lint-setup skill to set up the architecture rules for <language>`, start the `lint-setup` skill with that language as its argument, one language at a time. The skill stops and tells the user when the repository already has a manifest for another language.

## Route the arguments

- `status` or empty: run `harness run status` and continue the open run.
- `approve`: ask the `Plan` question (phase 3 below). A plain reply in the chat records nothing.
- `abandon`: run `harness run abandon` and report.
- Any other text with no active run: run `harness run start "<text>"`.

Each `run` command prints the phase and a `next:` line. Do the `next:` step.

## Phases

1. `planning`: start the `explorer` agent on the model of the `explorer` role in `.harness.yaml`. Give it the prompt. It reads `CONTEXT.md`, `docs/adr/` and the code that the change touches, and returns the facts and the open decisions.
2. `planning`: write `plan.md` in the run folder. `harness plan --help` gives the format. Put each open decision under `## Open questions`, or `- none`. Write each diagram through `harness diagram`. Run `harness plan check`, then `harness run plan`.
3. `awaiting_approval`: show the plan (`harness plan view` gives the page). Grill the open questions in rounds with the question tool of the host: `references/grill.md`. When `## Open questions` holds only `- none`, ask the `Plan` question with the options `Approve` (first) and `Change the plan`. Stop until the user answers.
   - On `Approve`, the hook records the approval for the current `plan.md`. Run `harness run approve`.
   - On `Change the plan`, ask what to change, edit `plan.md`, and ask the `Plan` question again.
   - `harness run approve` refuses with no record, after an edit of `plan.md`, on a plan check problem, and on an open question. Only when the user names a model for this run, run `harness run approve --by <model> --reason "<one line>"`.
4. `working`: for each task, write the failing test first, then the code. Run `harness verify` until each gate and each criterion passes. Commit, run `harness verify --commits <default branch>`, then `harness run review`.
5. `reviewing`: start the `reviewer` agent on the diff, on the model of the `reviewer` role in `.harness.yaml`. With findings, run `harness run rework --findings "<summary>"` and go back to `working`. With none, run `harness run ship`.
6. `shipping`: start the `pr-open` skill. It pushes the branch and opens the pull request as a draft, with the body Why, What and How. The user merges it. Then run `harness run done --pr <url>`.

A plan with only `docs` commits and no open question skips the approval: `harness run plan` moves it to `working`.

A change to the approved plan needs `harness run amend` and a new `Plan` answer.

## Guard

The guard denies some calls, for example a merge, a forced push, a read of a secret or a push before `verify --commits`. Read the deny reason and do the step that it names. Do not work around a deny with another command.
