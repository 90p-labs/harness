# 0018 Review round in four steps

## Status

Accepted, 2026-11-09. It replaces the round rule of ADR 0016 and the pre-ship panel rule of ADR 0017. The ship rule `review-models` and the floor of 2 models stay. Superseded in part by ADR 0020, 2026-11-10: the strict minor rule and the item R3b. Amended 2026-11-11: one verifier and one re-review for each round.

## Context

The counts below come from an audit of 50 rework reasons from past runs. The audit data is outside the repository. Each reason has a class and a severity.

- By class: 32 implementation, 6 spec-gap, 5 gate-checkable and 7 minor.
- By severity: 3 blocker, 15 major and 32 minor.
- A gate can check 5 of the reasons. A reviewer should not report them.

32 of the 50 reasons have the severity minor. Each rework moves `HEAD`, so the next round starts again.

Some reviews were shallow. 11 of 77 sonnet reviews used 3 API calls or fewer.
A reviewer that reports one finding at a time also costs one round for each finding.

## Decision

The round has 4 steps and one final panel.

1. **Common list**: one `reviewer` agent for each listed model, in parallel, on the same `HEAD`. Each reviewer lists every finding of its lens in one pass.
2. **Verify**: one `verifier` agent for each finding, in parallel. The lead drops each finding that the verifier calls not real.
3. **Fix**: the lead runs `harness run rework` once for the whole verified list. The lead starts one `worker` agent for each blocker and major finding.
4. **Re-review**: one `reviewer` agent checks each fixup diff. It answers `closed` or `open`.

After the re-reviews, the lead squashes the fixups and runs `harness run review`. The next common list is the final panel.

- **Final panel**: the common list runs once more on the final `HEAD`, with every listed model. After a small fixup, the smaller panel of ADR 0017 runs. When the first common list has no real finding, it already covers `HEAD`, and the lead ships.
- Lens Spec runs on the first model, lens Standards on the second, and lens Spec on every other model. Lens Standards does not report a rule that a gate checks.
- The verifier is a new agent file, on the model of the `explorer` role. It is not a mode of the reviewer, because a `reviewer` agent call in `reviewing` writes a review record. A haiku verifier would count as the haiku review of `HEAD`.
- The role set is fixed in code, so the verifier uses the model of the `explorer` role. The guard rule `agent-model-routing` leaves the agent type `verifier` unmapped, so it allows any model.
- The fix and the re-review run in `working`, after `harness run rework`. The review recorder acts only in `reviewing`, so they write no review record.
- The worker never commits. The lead makes one fixup for each target commit, the commit of the task that owns the file.
- The lead squashes with `git rebase -i --autosquash --keep-base`, then runs `harness verify --commits` and `harness run review`.
- Minor findings keep the strict rule until R3b, the next run, whose 2 items are listed below. The strict rule says that a real minor finding blocks the ship until a fix closes it.
- A real minor finding goes into the same rework, and one worker fixes all of them after the blocker and major fixes.
- The round limit `run.max_review_rounds` stays, with the default 3. With the strict minor rule, 2 reworks are possible before the run blocks.
- The ship rule stays: `review-models` denies `harness run ship` until every listed model of one host has a record on `HEAD`. The floor of 2 models stays.

Run R3b does 2 items:

1. Map `verifier` in `agentRoles` and add a `verifier` role.
2. Store the files that the reviewer read in the review record. Refuse a review that read no changed file.

## Consequences

- The reviewer reports each finding once. A rework fixes the whole verified list, so the number of rounds falls.
- The verifier costs one cheap call for each finding. It stops a false finding from causing a fix.
- The final panel adds one common list after a rework. Opus still reviews each final `HEAD`.
- The guard does not check the model of a verifier. A verifier call that uses a wrong model passes.

## Amendment 2026-11-11

Step 2 and step 4 now run once for each round, not once for each finding or fixup.

- **Verify**: one `verifier` agent gets all findings of the round and returns one line for each finding. A finding with no verifier line counts as real, and the lead keeps it.
- **Re-review**: one `reviewer` agent gets all finding lines and all fixups of the round, with `git show <sha>` for each. It answers `closed` or `open` for each finding and lists each new defect.

The reason is cost. Each agent start loads the brief and the diff again. One call for a round makes fewer agent starts, and the verdicts stay the same.
