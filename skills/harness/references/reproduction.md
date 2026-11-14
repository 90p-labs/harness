# Reproduce a bug before the plan

A bug cause is one of three things. The first is a prompt that reports broken behaviour. The second is an item of the evidence baseline with the class `bug` that the run fixes. The third is a plan that will hold a `fix` commit.

The plan of a bug cause has `Cause: bug` and needs a reproduction: one command that fails for the reported reason. The lead writes no plan until that command is red. The step runs in `planning`, after the explorer and before `plan.md`.

This step adapts the diagnosing-bugs skill of Matt Pocock (MIT license): <https://github.com/mattpocock/skills/tree/main/skills/engineering/diagnosing-bugs>. The reproduction gate, the ranked hypotheses and the seam rule come from that skill. The text here is new.

## The command

The command is the proof of one criterion, so the plan holds it once. It has these properties:

- It is already run. The worker saw it fail before it reports.
- It is red for the reported symptom, not for a build error or a missing test.
- It is deterministic. The plan check cannot show this, so the worker runs it twice.
- It is fast. Prefer one test with a short output, because the check reads only the last 4096 bytes of the output.
- The agent can run it with no human step.

For a bug in harness itself, the form of a reproduction is section 1 of `skills/report/SKILL.md`. Follow it and do not copy it here.

## The plan lines

Under `## Goal`, write `Cause: bug`. Then write `## Reproduction` with these lines:

- `- Criterion: n` names the criterion whose proof is the command.
- ``- Symptom: `text` `` is the text that the failed output holds. Use 8 characters or more. Do not start it with `exit status`, `--- FAIL`, `FAIL:`, `FAIL ` or `build failed`.
- ``- Regression test: `TestName` `` names the lasting test. `- Regression test: none: reason` says that no seam exists.
- 3 to 5 lines `n. If cause, then prediction. Result: confirmed.`

The rule `reproduction` checks the form. Then run `harness verify --criterion <n>` and read the output. The rule `reproduction-red` refuses `harness run plan` until the proof failed on the current files and printed the symptom. A setup failure or a gate veto saves no record, so read the output of the verify command.

## The hypotheses

Write 3 to 5 hypotheses, ranked from the most likely. Each one is falsifiable: it makes a prediction that the worker can see fail. Test them one variable at a time, so that one result has one cause. The plan check cannot see the experiments, so this stays guidance.

The result of each line is `confirmed`, `refuted` or `untested`. One line is `confirmed`, and no `untested` line ranks above it.

## No seam and human only

The seam rule: write the regression test at the point where the bug enters the code. When no seam reaches it, write `- Regression test: none: <reason>`. `harness run learn` then lists a `no-seam` candidate. Its home is a follow-up `harness run start` prompt that builds the seam, or an ADR for a deliberate gap.

When only a human can reproduce the bug, write `- Human only: <reason>` in place of `Criterion` and `Symptom`. No red check runs, and a hypothesis may stay `untested`. Name this in the `Plan` question, because no red proof backs the plan.

## A regression

When a bug returns, the run is in `planning` again and its old proof passes. The section then names a new red criterion for the new failure.

## The brief of the reproduction worker

The lead starts one `worker` agent and gives it the prompt, the likely command and seam, and the full path of the harness binary. The worker runs the raw command, because `plan.md` has no criterion gate yet.

- It writes only the reproduction file. The file stays uncommitted until `working`, when one task owns it.
- It reverts each experiment, so the tree holds no change but the reproduction file.
- It reports the command, the red tail and the hypotheses with their results.
- It stops and reports when it cannot make the command red. The lead then asks the user.
