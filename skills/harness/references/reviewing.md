# Reviewing the round

This is phase 5 of the harness loop, `reviewing`. Read it on entering `reviewing`, and after `harness run rework`. Run the steps below in order. After a rework, the next list is the final panel: `harness review panel` prints its models. Start each agent in a fresh context.

- **Common list**: start one `reviewer` agent for each listed model, in parallel, on the same `HEAD`. Set the `model` field of each agent call.
- Lens `Spec` runs on the first model of the `reviewer` role, lens `Standards` on the second, and lens `Spec` on every other model. Set the `model` field for each. Give each reviewer, also of the final panel, its lens, the path of `plan.md`, the default branch and the full path of the harness binary.
- Each reviewer returns every finding of its lens in one pass. The lead merges the lists into `findings.md` in the run folder, removes the duplicates and numbers them `F1`, `F2`.
- `findings.md` holds one or more tables. The columns `ID`, `Severity`, `Verdict` and `Status` are required. The other columns are free. The severity is `blocker`, `major` or `minor`. The verdict is `real` or `not real`. The status is `open` or `closed`.
  ```text
  | ID | Severity | Verdict | Status | Model | File:line | Finding |
  |----|----------|---------|--------|-------|-----------|---------|
  | F1 | major | real | open | opus | internal/a.go:12 | The error is dropped. |
  | F2 | minor | not real | closed | sonnet | docs/b.md:3 | The name differs. |
  ```
- The lead writes the `Model` cell: the model of the reviewer that raised the finding, copied from the `reviewer` role list. The final panel shrinks after a rework only when each reworked blocker or major finding has a `Model` cell naming a listed model. A missing or unlisted `Model` cell gives the full panel.
- The verifier sets `Verdict`. The lead sets `Status` to `closed` after a fix and its re-review, or after a minor fixup.
- The lead does not commit while a reviewer runs. The recorder reads `HEAD` when the reviewer stops.
- A review counts only when its transcript shows that the reviewer read a changed file. A `Read` call, a diff header in a tool result or a shell command that names the file all count.
- A reviewer with no result gets one new `reviewer` agent, with the same `model` field. A second gap is not a pass.
- **Verify**: start one `verifier` agent for all the findings of a round, with the `model` field set to the model of the `explorer` role. It returns one line for each finding, and a finding with no verifier line counts as real. The lead drops each finding that it calls not real, and takes the severity of the verifier.
- With no real open finding, run `harness run ship`. The guard needs a review of every model of the final panel, and 2 or more models. `harness review panel` prints the panel. The rule `review-findings` refuses a ship while `findings.md` has a real open blocker or major finding.
- With real findings, run `harness run rework --findings "<id, severity, file:line for each>" --class <implementation|spec-gap|gate-checkable|minor>` once for the whole list. Then go to `working`.
- The command reads `findings.md`. A real open blocker or major finding raises `review_round` by 1, once for the whole list. Only real open minor findings keep `review_round` and give the event `rework-minor`. A file with no real open finding is refused.
- With `run.strict_minor: true`, a minor finding counts like a major one. An amend keeps `review_round`. Only a regress resets it.
- **Minor fixup path**: with the default rule, the lead fixes each real minor finding as a fixup after the review. It may also record a known limit or a PR note and leave the status `open`. Then the lead runs `harness verify --commits <default branch>` and `harness run ship`, with no new review.
- The reviews of the run still count after `HEAD` changes, until the last `request-rework`, `amend` or `regress` of the run. This holds only while `findings.md` exists in the run folder. A clean rebase keeps the reviews by patch id, also with the strict rule. A change of whitespace only changes the patch id.
- Every real blocker or major finding goes through `harness run rework`. The command refuses a `--findings` text that does not name each real open blocker or major ID. The rule `review-findings` refuses a ship when a real blocker or major finding has an ID that no `request-rework` reason names. An ID matches as a whole word: `F1` does not match `F10`.
- Known limit: a lead that edits `findings.md`, for example sets a major to minor, can still pass the rule.
- **Fix** (in `working`): start one `worker` agent for each blocker and major finding. Set the `model` field to the first model of the `worker` role. Give each fix worker its finding lines, the path of `plan.md` and the full path of the harness binary.
- Start blockers first. Run in parallel only when the files do not overlap.
- The lead fixes with git a real finding that no worker can fix, such as the commit order.
- Each worker fixes only its finding, test first where a test can show it. It does not commit.
- The lead commits each fix with `git commit --fixup=<sha>`, one fixup for each target commit: the commit of the task that owns the file. A file outside the plan trees needs `harness run amend` first.
- One more `worker` agent, with the same `model` field, fixes all real minor findings after the blocker and major fixes.
- **Re-review** (in `working`): start one `reviewer` agent for all the blocker and major fixups of a round. Set the `model` field to the first model of the `reviewer` role. Give it the path of `plan.md`, the default branch and the full path of the harness binary.
- The reviewer gets all the finding lines of the round and runs `git show <sha>` for each fixup. It reads each diff and the changed files around it.
- The lead adds each new defect of a re-review to `findings.md`. It goes through Verify and Fix like the other findings.
- A fix that fails its re-review goes once to a new `worker` agent, with the same `model` field.
- Then run `GIT_SEQUENCE_EDITOR=: git rebase -i --autosquash --keep-base <default branch>`, `harness verify --commits <default branch>` and `harness run review`.
- **Final panel**: run `harness review panel`. Start one `reviewer` agent for each model that it prints, with the `model` field set to that model, on the final `HEAD`. After a small fixup it can be fewer than every listed model. Its findings go through Verify and a new rework.
- The limit is `run.max_review_rounds` (default 3). Past it, the run blocks.
