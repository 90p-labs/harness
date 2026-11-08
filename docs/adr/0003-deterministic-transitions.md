# 0003 Deterministic transitions

## Status

Accepted, 2026-11-07. Updated 2026-11-08 for the trace.

## Context

Kodify has several target phases for one phase and event. The outcome depends on code outside the table. The loop also ends at the PR, so production signals never return.

## Decision

- The transition table holds guarded rows. For each phase and event, exactly one row applies, and a test proves it.
- A regression sends the same run back to `planning`. The agent reconsiders the whole plan for the impact of the regression.
- The go-live path has two phases, `releasing` and `observing`. The watch window starts at the deploy. A phase is simpler than a flag.
- Each plan carries a trace. The review verifies each link of the trace before the run moves to `reviewing`.
- PR 4 holds the trace. PR 7 holds the criterion gates and the review veto. PR 10 only draws them.

## Consequences

- `DESIGN.md` draws the table, and a root test compares the drawing with `domain.Transitions()`.
- An illegal event returns `ErrIllegalTransition`, never a second target.
- A regression keeps the run history in one run, with a live round counter.
- The review can refuse a plan with a broken trace link.
