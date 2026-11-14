# 0024 Learn conventions at ship

## Status

Accepted, 2026-11-13. It adds the `harness run learn` command and the `run.learn` span. It adds no phase and no event, so the state machine and the plugin events do not change. The amendment below adds the run evidence and the lessons. A second amendment adds the lesson trace: the follow-up entries of `docs/follow-ups.md` and the draft for the `report` skill.

## Context

- A reviewer finds the same kind of problem in run after run. The fix lands each time, but no rule records it, so the next run repeats the mistake.
- The review findings of a run are in `findings.md`. Some of them are real problems that a rule of the project would prevent. The binary does not know which ones.
- The user also corrects the lead during a run, such as "use this helper, not that one". The binary has no record of these corrections. They are in the session and in `handoff.md`.
- A rule written at a later time, in a separate run, loses the context of the finding. A rule written at ship, while the run is still open, keeps it.
- A repeated rework class, such as `implementation`, names one-off bugs of the diff. It carries no rule text, so it is not a source of a rule.

## Decision

1. `harness run learn [<run id>]` is a read-only query. It prints one `candidate:` line for each candidate and a `next:` line. It adds no phase and no event. Its span is `run.learn`, with `harness.run.id`.
2. The step runs in the phase `shipping`, before `pr-open`. The conventions commit is then part of the pull request.
3. A candidate is a real finding of lens `standards`. A candidate is also a real finding that the reason of a `gate-checkable` rework names. The other rework classes and the Spec lens give one-off bugs of the diff, so they are not candidates.
4. The lead adds the user corrections as candidates of source `user`. The binary cannot read them, so the lead lists each one from its session and from `handoff.md`.
5. The lead asks the user one question, with the header `Conventions` and the options `Write` and `Skip`. The host adds a free-text answer, which drops or changes candidates. One round covers all the candidates.
6. A confirmed convention goes to the first home that fits, in this order:
   - a lint rule. The lead prints a follow-up `harness run start` prompt for the rule and its canary. The lead writes no lint file in this run.
   - an ADR and its entry in `docs/adr/index.md`.
   - an entry in `CONTEXT.md`.
7. Before a write, the lead searches `CONTEXT.md`, `docs/adr/` and the lint pack for the rule. If a rule exists, the lead extends it and does not add a second one.
8. The next run reads the conventions. The explorer reads `docs/adr/index.md` and `CONTEXT.md`. The lens Standards and the `standards-rules` check name `docs/adr/` and `CONTEXT.md`.

## Consequences

- A rule that a run finds is written before the pull request opens, so the next run can read it.
- The user confirms each convention before the write. A user who answers `Skip` leaves the repository unchanged.
- The command is read-only. It can run again without a side effect, and it needs no new state.
- A candidate can be missed when the lead does not list a user correction. The `Conventions` question is the place where the user can add one.
- A `findings.md` with no `Lens` column gives no Standards candidate. The reviewing reference shows the column, so new runs fill it.
- A light run with no `findings.md` prints no candidate and does not fail the step.
- A lint rule waits for its own run and review. The conventions commit does not change the lint pack.

## Amendment 2026-11-14

The step asked which rule the code broke. It did not ask what slowed the agent down. The amendment adds that question.

- The step extends `harness run learn`. Both lists come from one read of the run record and share one question.
- A second command would add a second step and a second question. No decision above is replaced, so no new ADR is needed.
- The command also prints one `evidence:` line for each item of the run evidence. It reads the transition log and the run row.
- The IDs are `rework-<n>` for each review rework, `block-<n>` for each block, `amends` and `stop-blocks`.
- A rework line holds its class and its reason. A block line holds its reason. `amends` and `stop-blocks` hold a count.
- The command does not read gate failures, because the gate cache has no run id. It does not read regressions, which have their own plan section.
- The binary does not parse `handoff.md`. The lead reads it, and may cite one of its lines.
- A lesson is a change of the agent environment that cites run evidence and has a home.
- Navigation: what was hard to find, and the pointer that finds it.
- Automated checks: a check that no gate runs, or a mechanical mistake.
- Tool economy: a command or a tool call that was repeated or wasted.
- No-ops: an instruction that changed nothing.
- Information access: something the agent needed and could not reach.
- Evidence rule: a lesson cites an `evidence:` line, a `candidate:` line, a row of `findings.md` or a line of `handoff.md`. A lesson with no evidence is dropped.
- The memory of the lead is not evidence.
- A count line alone supports no lesson, because most runs have one. The lesson also cites a line with text.
- Mechanical rule: a mechanical mistake gets a deterministic check, never a sentence of guidance.
- A repeated rework class supports a lesson. It is still not the source of a convention.
- A check, a gate, a script or a setting goes to a follow-up `harness run start` prompt.
- A pointer or a removed instruction goes into the `docs:` commit of this run.
- A Navigation pointer goes into `CONTEXT.md`, or into the line of its ADR in `docs/adr/index.md`, because the index holds one line for each ADR.
- The no-duplicate search of decision 7 covers a lesson too.
- A lesson about the harness plugin has no home in the repository. The lead prints it in one line of its last message.
- The `Conventions` question keeps its header and options. It lists the lessons after the conventions, so one round covers both.
- The step takes the idea from the retro skill of Matt Pocock (MIT license), at commit `5863806`. The text here is new.
- The step leaves out two source categories. Coding standards is covered by decisions 3 and 6, and global agent files are covered by No-ops.

## Amendment 2026-11-14, lesson trace

A replay on 6 past runs found two runs with one follow-up prompt that nobody started. The prompt was printed once and then lost. The lesson for the harness plugin had the same loss, in one line of the last message.

- Each follow-up prompt gets an entry in `docs/follow-ups.md`. The lint rule prompt of a convention gets one too, because it is the same printed prompt. The `no-seam` prompt of ADR 0025 gets an entry too.
- An entry holds the cause in a heading, then a `Category` line, a `Cited by` line and a `Prompt` line.
- A `Cited by` line holds the run id and one evidence ID or `file:line`, each in a code span. One line stands for one run.
- On a repeat, the lead adds one `Cited by` line to the entry and writes no second entry. The count of lines shows the repeat.
- The prompt asks the follow-up run to remove the entry. The file then lists only open follow-ups, and the git history keeps the closed ones.
- The home is a file and not an issue. A file costs no network call and no login, and it works on any host.
- The entry lands in the `docs:` commit, so the review of the run covers it. An issue is public before any review.
- Known cost: the entry reaches the next run only after the merge. A run that started earlier sees it after its rebase.
- Known cost: two runs that add an entry in parallel conflict at the rebase. The lead merges two entries of one cause there.
- The no-duplicate search of decision 7 also reads the open causes of `docs/follow-ups.md`. One call for the whole step lists them, and the lead compares by cause.
- This amendment replaces the last home of the first amendment. A lesson about the harness plugin becomes a draft for the `report` skill, and the confirmation of that skill decides the filing.
- A plugin lesson is not in the `Conventions` question. Its draft changes no file of the repository, and the `report` skill asks its own question.
- The lead keeps the draft before the `Conventions` question, as `report-<n>.md` in the run folder, and names it in `handoff.md`. It asks for the confirmation after `harness run wait --pr <url>`, because the stop rule allows a stop in `shipping` only when the run holds a pull request URL.
- Unattended means that the user told the lead to ask nothing, or that the session has no user. The draft then stays in the run folder, and the last message lists its path.
- A repeat of a plugin lesson becomes a comment on its issue, kept and confirmed like a report draft.
- Git ignores `.harness/`, so the draft ends with the worktree. The last message and `handoff.md` give its path before that.
- This amendment extends decisions 6 and 7. It replaces no numbered decision.
