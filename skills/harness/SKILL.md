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

- `status` or empty: run `harness run status` and continue the open run.
- `approve`: ask the `Plan` question (phase 3 below). A plain reply in the chat records nothing.
- `abandon`: run `harness run abandon` and report.
- Any other text with no active run: run `harness run start "<text>"`.

Each `run` command prints the phase and a `next:` line. Do the `next:` step.

## Phases

1. `planning`: start the `explorer` agent. Set the `model` field of the agent call to the model of the `explorer` role in `.harness.yaml`. Give it the prompt. It reads `CONTEXT.md`, `docs/adr/` and the code that the change touches, and returns the facts and the open decisions.
2. `planning`: on Claude, compare the session model with the model of the `lead` role (`harness config`). When they differ, tell the user to run `/model` to switch, and continue: it is guidance, not a deny. Then write `plan.md` in the run folder. `harness plan --help` gives the format. Put each open decision under `## Open questions`, or `- none`. Write each diagram through `harness diagram`. Run `harness plan check`. Then start the `advisor` agent, with the `model` field set to the model of the `advisor` role. It reads `plan.md` and the code that the plan touches. Fix `plan.md` for each finding that you accept. Then run `harness run plan`.
3. `awaiting_approval`: show the plan (`harness plan view` gives the page). Grill the open questions in rounds with the question tool of the host: `references/grill.md`. When `## Open questions` holds only `- none`, ask the `Plan` question with the options `Approve` (first) and `Change the plan`. Stop until the user answers.
   - On `Approve`, the hook records the approval for the current `plan.md`. Run `harness run approve`.
   - On `Change the plan`, ask what to change, edit `plan.md`, and ask the `Plan` question again.
   - `harness run approve` refuses with no record, after an edit of `plan.md`, on a plan check problem, and on an open question. Only when the user names a model for this run, run `harness run approve --by <model> --reason "<one line>"`.
4. `working`: you are the driver. You do not write the code of a task. `harness plan view` gives the waves.
   - A light-track run can use `sonnet` as the lead. The track shows in `harness plan view`. On a light track, the lead does not ask a `sonnet` session to switch.
   - For each wave, start one `worker` agent for each task, in the background, with the `model` field set to the `Model` of the task. The model must be in the `worker` role list. Start the tasks of a wave in parallel when their owned files do not overlap.
   - Use low effort for a haiku task that is mundane: docs, a rename or a config change.
   - Check each result: the owned files (`git diff --stat`), the red and green lines, and `harness verify`.
   - Read a diff with `git diff --stat` first, then only the files that you check. Read a long output through `tail`, with `set -o pipefail` so that the exit code stays. Example: `set -o pipefail; go test ./... 2>&1 | tail -n 40`.
   - Send a failed task once to a new `worker` agent, with the same `model` field.
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
   - The worker fixes each miss test first. The lead commits each fix with `git commit --fixup=<sha>`, rebases with autosquash, and runs `harness verify --commits <default branch>` again. An `open` check goes to a fix worker.
   - Then the lead sets `Head` to the new `HEAD` and runs `harness run review`. The command refuses a run folder with no `self-check.md` or no `Head: <sha>` line.
   - After `harness run rework`, do the steps Fix and Re-review of phase 5.
5. `reviewing`: run the steps below in order. Start each agent in a fresh context.
   - **Common list**: start one `reviewer` agent for each listed model, in parallel, on the same `HEAD`. Set the `model` field of each agent call.
   - Lens `Spec` runs on the first model of the `reviewer` role, lens `Standards` on the second, and lens `Spec` on every other model. Set the `model` field for each.
   - Each reviewer returns every finding of its lens in one pass. The lead merges the lists into `findings.md` in the run folder, removes the duplicates and numbers them `F1`, `F2`.
   - `findings.md` holds one or more tables. The columns `ID`, `Severity`, `Verdict` and `Status` are required. The other columns are free. The severity is `blocker`, `major` or `minor`. The verdict is `real` or `not real`. The status is `open` or `closed`.
     ```text
     | ID | Severity | Verdict | Status | File:line | Finding |
     |----|----------|---------|--------|-----------|---------|
     | F1 | major | real | open | internal/a.go:12 | The error is dropped. |
     | F2 | minor | not real | closed | docs/b.md:3 | The name differs. |
     ```
   - The verifier sets `Verdict`. The lead sets `Status` to `closed` after a fix and its re-review, or after a minor fixup.
   - The lead does not commit while a reviewer runs. The recorder reads `HEAD` when the reviewer stops.
   - A review counts only when its transcript shows that the reviewer read a changed file. A `Read` call, a diff header in a tool result or a shell command that names the file all count.
   - A reviewer with no result gets one new `reviewer` agent, with the same `model` field. A second gap is not a pass.
   - **Verify**: start one `verifier` agent for each finding, in parallel, with the `model` field set to the model of the `explorer` role. The lead drops each finding that it calls not real, and takes the severity of the verifier.
   - With no real open finding, run `harness run ship`. The guard needs a review of every listed reviewer model, and 2 or more models. The rule `review-findings` refuses a ship while `findings.md` has a real open blocker or major finding.
   - With real findings, run `harness run rework --findings "<id, severity, file:line for each>" --class <implementation|spec-gap|gate-checkable|minor>` once for the whole list. Then go to `working`.
   - The command reads `findings.md`. A real open blocker or major finding raises `review_round` by 1, once for the whole list. Only real open minor findings keep `review_round` and give the event `rework-minor`. A file with no real open finding is refused.
   - With `run.strict_minor: true`, a minor finding counts like a major one. An amend keeps `review_round`. Only a regress resets it.
   - **Minor fixup path**: with the default rule, the lead fixes each real minor finding as a fixup after the review. It may also record a known limit or a PR note and leave the status `open`. Then the lead runs `harness verify --commits <default branch>` and `harness run ship`, with no new review.
   - The reviews of the run still count after `HEAD` changes, until the last `request-rework`, `amend` or `regress` of the run. This holds only while `findings.md` exists in the run folder. A clean rebase keeps the reviews by patch id, also with the strict rule. A change of whitespace only changes the patch id.
   - Every real blocker or major finding goes through `harness run rework`. The command refuses a `--findings` text that does not name each real open blocker or major ID. The rule `review-findings` refuses a ship when a real blocker or major finding has an ID that no `request-rework` reason names. An ID matches as a whole word: `F1` does not match `F10`.
   - Known limit: a lead that edits `findings.md`, for example sets a major to minor, can still pass the rule.
   - **Fix** (in `working`): start one `worker` agent for each blocker and major finding. Set the `model` field to the first model of the `worker` role.
   - Start blockers first. Run in parallel only when the files do not overlap.
   - The lead fixes with git a real finding that no worker can fix, such as the commit order.
   - Each worker fixes only its finding, test first where a test can show it. It does not commit.
   - The lead commits each fix with `git commit --fixup=<sha>`, one fixup for each target commit: the commit of the task that owns the file. A file outside the plan trees needs `harness run amend` first.
   - One more `worker` agent, with the same `model` field, fixes all real minor findings after the blocker and major fixes.
   - **Re-review** (in `working`): start one `reviewer` agent for each blocker and major fixup. Set the `model` field to the first model of the `reviewer` role.
   - The reviewer gets the finding line and `git show <fixup sha>`.
   - The lead adds each new defect of a re-review to `findings.md`. It goes through Verify and Fix like the other findings.
   - A fix that fails its re-review goes once to a new `worker` agent, with the same `model` field.
   - Then run `GIT_SEQUENCE_EDITOR=: git rebase -i --autosquash --keep-base <default branch>`, `harness verify --commits <default branch>` and `harness run review`.
   - **Final panel**: the next common list runs once on the final `HEAD`, with every listed model. Its findings go through Verify and a new rework.
   - The limit is `run.max_review_rounds` (default 3). Past it, the run blocks.
6. `shipping`: start the `pr-open` skill. It pushes the branch and opens the pull request as a draft, with the body Why, What and How. The user merges it. Then run `harness run report`, then `harness run done --pr <url>`.

A plan with only `docs` commits and no open question skips the approval: `harness run plan` moves it to `working`.

A change to the approved plan needs `harness run amend` and a new `Plan` answer.

## Guard

The guard denies some calls, for example a merge, a forced push, a read of a secret or a push before `verify --commits`. Read the deny reason and do the step that it names. Do not work around a deny with another command.

On Codex, the agent call has no `model` field. Leave it out: the guard lets the call pass.
