# 0020 Review round counts blocker and major

## Status

Accepted, 2026-11-10. It replaces the strict minor rule and the item R3b of ADR 0018.

## Context

- An audit of 50 reworks found 32 with only minor findings. Each of them raised the review round.
- An amend with approval reset the round, so the limit did not stop a long loop.
- A rebase onto a moved default branch voided the reviews of the old `HEAD`, so a clean rebase needed a new panel.
- The recorder wrote a review at the dispatch of a background agent. That record held no report and no files read.

## Decision

1. `findings.md` is a table with the columns ID, Severity, Verdict and Status. Severity is `blocker`, `major` or `minor`. Verdict is `real` or `not real`. Status is `open` or `closed`.
2. `harness run rework` reads `findings.md`. A real open blocker or major finding gives `request-rework` and raises `review_round` by 1, once for the whole list. Only real open minor findings give `rework-minor`, which keeps the round. A file with no real open finding is refused.
3. With `run.strict_minor: true`, a minor finding counts like a major one. An approve after an amend keeps the round. Only a regress resets it.
4. The guard rule `review-findings` refuses `harness run ship` while `findings.md` has a real open blocker or major finding. With the strict rule, a real open minor finding also refuses.
5. The recorder writes a review when the reviewer stops (`subagent.stopped`). It stores the patch id and the files read. The rule `review-models` counts a review only when its transcript shows a read of a changed file. The deny names the model.
6. `harness run review` refuses a run folder with no `self-check.md` or no `Head: <sha>` line.

A review matches by `HEAD`, by patch id, or by run. The run match ends at the last `request-rework`, `amend` or `regress` of the run. It needs `findings.md` in the run folder. It is off with the strict rule. The patch id is `git patch-id --verbatim`, so a change of whitespace only changes the patch id.

So after a review, the lead fixes the minor findings as fixups and ships with no new review. A clean rebase keeps the reviews by patch id, also with the strict rule.

## Consequences

- With the default rule, 18 of the 50 audited reworks count as a round. With the strict rule, 50 count.
- Codex sends no model in an agent call, so it records no review. A Codex repository disables `review-models`. This is a known limit.
- The recorder reads `HEAD` at `subagent.stopped`. The lead does not commit while a reviewer runs.
- The check of a changed file matches the file name in the transcript. A reviewer that only names a file in a shell command counts.
- `harness run rework` refuses a `--findings` text that does not name each real open blocker or major ID. The rule `review-findings` refuses a ship while a real blocker or major finding, open or closed, has an ID that no `request-rework` reason names. An ID matches as a whole word.
- A lead that edits `findings.md`, for example sets a major to minor, can still pass the rule. This is a known limit.
- A diff header after the line-number prefix of a `Read` result counts as a read of that file.
- A real minor finding can stay open with a note in the PR, and the ship still passes.
