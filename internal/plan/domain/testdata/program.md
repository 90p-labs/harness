# Harness improvement plan

## Goal

Eight harness runs in 3 waves move each check to the step that makes the defect. The steps are the plan, the worker, a gate and the reviewer.
The targets: rework rounds from 1.6 to under 1.0 for each run, and denies from 10.8 to under 4 for each run.

## Runs

### R1 guard-precision

- Wave: 1
- Depends on: none
- Branch: feat/guard-precision
- Owned files: `internal/guard/domain/shell.go`, `internal/guard/domain/rules_files.go`, `internal/guard/domain/rules_pr.go`
- Done when: a replay of the denied commands matches the expected result after the fix.

1. Expand `NAME=value` assignments and follow `cd` before the source-path check.
2. Each deny names the resolved file and the line number.
3. Add `harness pr check --body-file <path>`, which prints all problems in one pass.

### R2 prose-gate

- Wave: 1
- Depends on: none
- Branch: feat/prose-gate
- Owned files: `internal/plan/domain/sentence.go`, `skills/skills_test.go`, `.harness.yaml`
- Done when: the word-count reasons of the rework log fail the gate in the working phase.

1. Move the sentence checker to a shared package, and add the paragraph rule.
2. Add a gate for the changed markdown files, with a baseline ratchet for old text.

### R3a review-brief

- Wave: 1
- Depends on: none
- Branch: docs/review-brief
- Owned files: `skills/harness/SKILL.md`, `agents/reviewer.md`
- Done when: a review of a sample diff returns all findings in one report with a severity on each.

1. The reviewer lists every finding of its lens in one pass, most severe first.
2. A re-review gets the old findings and the fix diff, and checks them first.

### R4 spec-quality

- Wave: 2
- Depends on: R2
- Branch: feat/spec-quality
- Owned files: `agents/explorer.md`, `agents/advisor.md`, `internal/plan/domain/check.go`
- Done when: a replay of two old plans gives the known gaps as open questions at plan time.

1. The explorer and the advisor check each fact of the plan against the code.
2. Each acceptance criterion has a proof command, and `harness verify` runs the proofs.
3. A plan template holds each rule that `plan check` enforces.

### R5 coordinator

- Wave: 2
- Depends on: R1
- Branch: feat/coordinator
- Owned files: `internal/run/ui/cli/list.go`, `internal/run/domain/approval.go`
- Done when: the lead watches all runs with 1 command, and a merge is 1 command.

1. Add `harness run list` for every open run of the shared state database.
2. The question-tool hook records the wait, and the answer clears it.
3. A merge script runs the merge checklist and refuses a worktree with uncommitted files.

### R6 telemetry-and-stop

- Wave: 2
- Depends on: R1 and R3a
- Branch: feat/telemetry-and-stop
- Owned files: `internal/run/domain/stop.go`, `internal/run/application/query/report/`
- Done when: the audit counts come from 1 Jaeger query and 1 command.

1. Add the tool and the rule to the hook span and to the deny span.
2. Add `harness run report <id>` with the rounds, the amends and the minutes in each phase.

### R3b review-loop

- Wave: 3
- Depends on: R3a and R4
- Branch: feat/review-loop
- Owned files: `internal/run/domain/guard.go`, `internal/run/infrastructure/sqlite/migrations/`
- Done when: a rebase after the review needs no new review, and a run with only minor findings ships after 1 round.

1. Add `harness run rework --severity blocker|major|minor`. The round limit counts only blocker and major rounds.
2. Store the stable patch id with each review record.

### R7 lead-cost

- Wave: 3
- Depends on: R1, R2 and R4
- Branch: feat/lead-cost
- Owned files: `skills/lead/SKILL.md`, `internal/verify/domain/output.go`
- Done when: the median context of a run lead call falls by 30% on 3 runs.

1. `harness verify` prints only the failed gates and 1 summary line by default.
2. The skill tells the lead to read diffs with `--stat` first.

## Measurement

`harness run report` and the state database give the numbers after each wave.

| Metric | Baseline | Target |
|---|---|---|
| Rework rounds for each run | 1.6 | under 1.0 |
| Round-limit blocks | 9 | 0 |
| Denies for each run | 10.8 | under 4 |
| Unseen user waits over 15 minutes | 6 or more | 0 |

## Decisions

- Minor rule: R3b turns it on as the default once R2 and R4 have merged.
- Unattended runs: both approval ways stay legal.
- The coordinator runs at most 3 runs at the same time.
