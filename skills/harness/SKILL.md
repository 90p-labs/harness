---
name: harness
description: "Drives one change through the harness loop: plan, approval, test-first work, verify gates, review and ship. Use when the user types /90p:harness (Codex: $90p:harness) with a prompt, or with status, approve or abandon, in a repository that has .harness.yaml."
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

- `status` or empty: run `harness run status`. When the run folder has a `handoff.md`, read it. Continue the open run from both.
- `approve`: ask the `Plan` question (phase 3 below). A plain reply in the chat records nothing.
- `abandon`: run `harness run abandon` and report.
- Any other text with no active run: run `harness run start "<text>"`.

Each `run` command prints the phase and a `next:` line. Do the `next:` step. When a `handoff:` line asks for it, write `handoff.md` in the run folder as a checkpoint, then continue with the `next:` step. Do not end the turn, except for a question to the user, such as the `Plan` question, or when your context is near its limit. In `shipping`, the turn may also end while the run waits on the merge after `harness run wait`. A new session resumes from `harness run status` and `handoff.md`.

## Phases

1. `planning`: start the `explorer` agent. Set the `model` field of the agent call to the model of the `explorer` role, from the `models:` line of `harness run start` or `harness run status`. Give it the prompt. It reads `docs/adr/index.md` first, then `CONTEXT.md`, the ADRs that the index names and the code that the change touches. It returns the facts and the open decisions.
2. `planning`: on Claude, compare the session model with the model of the `lead` role in the `models:` line. When they differ, tell the user to run `/model` to switch, and continue: it is guidance, not a deny. Then write `plan.md` in the run folder. `harness plan --help` gives the format. Put each open decision under `## Open questions`, or `- none`. Write each diagram through `harness diagram`.
   - Run `harness plan check`. It prints an advisor line from the plan complexity, then one line for each wave of tasks. The wave lines print only for a clean single-run plan.
   - On `advisor: on, model <m>`, with or without the text `(light track, risky path)`, start the `advisor` agent, with the `model` field set to `<m>`, the model that the line names. It reads `plan.md` and the code that the plan touches. Fix `plan.md` for each finding that you accept.
   - On `advisor: off`, start no advisor. Decide each open point yourself and record each decision under `## Decisions` in `plan.md`.
   - Then run `harness run plan`.
3. `awaiting_approval`: show the plan. `harness run plan` wrote the page `plan.html` in the run folder, and it follows the run. The run opens the page in the browser on entering `awaiting_approval`. After each edit of `plan.md`, run `harness plan view` and ask the user to reload the page. Grill the open questions in rounds with the question tool of the host: `references/grill.md`. When `## Open questions` holds only `- none`, ask the `Plan` question with the options `Approve` (first) and `Change the plan`, then stop until the user answers.
   - On `Approve`, the hook records the approval for the current `plan.md`. Run `harness run approve`.
   - On `Change the plan`, ask what to change, edit `plan.md`, run `harness plan view`, and ask the `Plan` question again.
   - `harness run approve` refuses with no record, after an edit of `plan.md`, on a plan check problem, and on an open question. Only when the user names a model for this run, run `harness run approve --by <model> --reason "<one line>"`.
4. `working`: you are the driver. You do not write the code of a task. `harness plan check` gives the waves (only for a clean single-run plan) and the model of each task.
   - A light-track run can use `sonnet` as the lead. `harness plan check` prints `advisor: off (light track)` on a light track. The text `(light track, risky path)` on the `on` line marks a light-track plan with a guard, security or hook path. On a light track, the lead does not ask a `sonnet` session to switch.
   - For each wave, start one `worker` agent for each task, in the background, with the `model` field set to the `Model` of the task. The model must be in the `worker` role list. Start the tasks of a wave in parallel when their owned files do not overlap. Give each worker, also a fix worker, the path of `plan.md`.
   - Use low effort for a haiku task that is mundane: docs, a rename or a config change.
   - Check each result: the owned files (`git diff --stat`), the red and green lines, and `harness verify`.
   - Read a diff with `git diff --stat` first, then only the files that you check. Read a long output through `tail`, with `set -o pipefail` so that the exit code stays. Example: `set -o pipefail; go test ./... 2>&1 | tail -n 40`.
   - Send a failed task once to a new `worker` agent, with the same `model` field. A plan conflict in the report goes to the lead in place of a new worker. The lead trims the task or amends the plan.
   - Commit in plan order, one commit for each task.
   - Run `harness verify --commits <default branch>` in the foreground, with a Bash timeout of 600000 ms (600 s). When it does not fit in 600 s, run it in the background.
   - Self-check: start one `worker` agent with the `model` field set to the first model of the `worker` role. It reads `git diff <default branch>...HEAD`, the lens checklists of `agents/reviewer.md` and `plan.md`. It runs each proof command of `## Acceptance criteria`. A `|` in a proof command is written `\|` in the proof table.
   - The worker writes `self-check.md` in the run folder, in this format. The results are `pass`, `fixed` or `open`. The proof table has one row for each criterion.
     ```text
     # Self-check

     Head: <full sha of HEAD>

     | Check | Result | Evidence |
     |-------|--------|----------|
     | spec-proof | pass | <criterion or file:line> |
     | spec-scope | pass | <criterion or file:line> |
     | spec-text | pass | <criterion or file:line> |
     | standards-rules | pass | <criterion or file:line> |
     | standards-judgement | pass | <criterion or file:line> |

     | # | Proof | Exit |
     |---|-------|------|
     | 1 | `<proof command>` | 0 |
     ```
   - `spec-proof`: each criterion has its code, and its proof fails without the code.
   - `spec-scope`: each task changed only its owned files, with no missing or partial requirement and no scope creep.
   - `spec-text`: each changed doc sentence agrees with the code and with the other docs.
   - `standards-rules`: the diff follows `CLAUDE.md`, `AGENTS.md`, `CONTEXT.md` and the lint rules.
   - `standards-judgement`: edge cases, error paths, leaks, dead code and code smells.
   - The worker does not change a value, a name or a limit that `plan.md` sets. When a fix needs one, the worker writes `open` with `plan conflict:` in the evidence. The lead takes each such row, and trims the fix or amends the plan.
   - The worker fixes each miss test first. The lead commits each fix with `git commit --fixup=<sha>`, rebases with autosquash, and runs `harness verify --commits <default branch>` again. Each other `open` check goes to a fix worker.
   - Run plain `harness verify` after the last commit. It records the proofs that `harness run review` needs, and `verify --commits` does not.
   - Then the lead sets `Head` to the new `HEAD` and runs `harness run review`. The command refuses a run folder with no `self-check.md` or no `Head: <sha>` line.
   - After `harness run rework`, read `references/reviewing.md` and do its Fix and Re-review steps.
5. `reviewing`: on entering `reviewing`, and after `harness run rework`, read `references/reviewing.md` and follow it.
6. `shipping`: start the `pr-open` skill. It pushes the branch and opens the pull request as a draft, with the body Why, What and How. Then run `harness run wait --pr <url>`: the turn may stop while the run waits on the merge. After a `resume` into `shipping`, run the wait again. The user merges the pull request. Then run `harness run report`, then `harness run done --pr <url>`.

A plan with only `docs` commits and no open question skips the approval: `harness run plan` moves it to `working`.

To change the approved plan, edit `plan.md` first, then run `harness run amend`. When approval is needed, the run goes to `awaiting_approval`: ask the `Plan` question again, then run `harness run approve`. With `run.approval: false`, the run goes to `working` with no `Plan` answer. A tree-only amend skips the approval only in `working` with no live review round; from `reviewing` or `shipping`, it needs the `Plan` answer. `harness run plan` never follows an amend.

## Guard

The guard denies some calls, for example a merge, a forced push, a read of a secret or a push before `verify --commits`. Read the deny reason and do the step that it names. Do not work around a deny with another command.

On Codex, the agent call has no `model` field. Leave it out: the guard lets the call pass.
