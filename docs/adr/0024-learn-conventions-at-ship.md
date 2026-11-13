# 0024 Learn conventions at ship

## Status

Accepted, 2026-11-13. It adds the `harness run learn` command and the `run.learn` span. It adds no phase and no event, so the state machine and the plugin events do not change.

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
