# 0025 Reproduce a bug before the plan

## Status

Accepted, 2026-11-14. It adds the `Cause: bug` plan line, the `## Reproduction` section, the rules `reproduction` and `reproduction-red` and the flag `harness verify --criterion`. It adds no phase and no event, so the state machine and the plugin events do not change.

## Context

- A plan for a bug often names a cause that nobody saw. The fix then lands, and the test passes without proof that it covered the reported failure.
- A criterion gate exists only after `plan.md`, so the first reproduction cannot use it.
- The check of a plan cannot judge a fix. It can check that one proof failed for the reported reason.
- The idea comes from the `diagnosing-bugs` skill of Matt Pocock (MIT license): the reproduction gate, the ranked hypotheses and the seam rule. The text here is new.

## Decision

1. The line `Cause: bug` under `## Goal` marks the bug path. A run has no cause field, and a plan line is checkable. A plan with a `fix` commit needs it.
2. The section `## Reproduction` names one criterion and the `Symptom` text that its failed output holds. It holds a `Regression test` and 3 to 5 ranked hypotheses with a result. The proof of the criterion is the command, so the plan holds it once.
3. `Human only` replaces `Criterion` and `Symptom` when only a human can reproduce the bug. No red check runs. It adds no phase, no question and no approval rule.
4. `harness verify --criterion <n>` runs only the proof of one criterion and saves the evidence, so the agent gets a red record without the slow gates.
5. In `planning`, `harness run plan` applies the rule `reproduction-red`. It reads the evidence and runs nothing. It refuses a plan with no record for the current files, a proof that passed, or an output tail without the symptom. An amend keeps the static rule and runs no red check.
6. One `worker` agent builds the reproduction and tests the hypotheses in `planning`, so the lead keeps its context for the plan. The reproduction file stays uncommitted until `working`.
7. `Regression test: none: <reason>` records a missing seam. `harness run learn` lists it first, with the source `no-seam`. Its home is a follow-up `harness run start` prompt that builds the seam, or an ADR for a deliberate gap.
8. A regression of a bug run puts the run in `planning` again. Its old proof passes, so the section names a new red criterion.

## Consequences

- A bug plan has seen its failure before it is approved, and the regression test has a proof of the cause.
- A symptom check by substring is a narrow judgement. The lead still decides whether the symptom is the reported one.
- Determinism, speed and one variable at a time stay guidance in `references/reproduction.md`, because one evidence record cannot show them.
- A `Human only` plan has no red proof. The lead names it in the `Plan` question, and the line stays in `plan.md`.
- A setup failure or a gate veto saves no record. The message tells the agent to read the output of `harness verify --criterion`.
- An approved `fix` plan of an older binary fails the check after an upgrade. The amend adds the two parts. It needs the approval, because both parts count as structure.
