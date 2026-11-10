# 0017 Pre-ship review panel

## Status

Accepted, 2026-11-09. It replaces the third-reviewer rule of ADR 0016. ADR 0018 replaces its pre-ship panel rule. The ship rule needs every listed model of the host, and the floor of 2 models stays.

## Context

The page `docs/bench/review-models.md` compares two sides on 4 PRs:

- Side A, one opus reviewer: 11 of 15 confirmed defects and 3 of 3 majors. It cost 4.35 USD and took 1,751 s.
- Side B, sonnet and haiku, with opus as third reviewer on 2 of 4 PRs: 8 of 15 defects and 2 of 3 majors. It cost 2.10 USD and took 766 s.
- On PR 38, sonnet and haiku alone found the major in 1 of 6 runs. Opus found it in 5 of 6.
- The PR 38 series of 5 runs showed no measurable difference in cost and wall time.

The third reviewer joined only on a large diff or a named risk. The majors needed the strong model on every change.

## Decision

- Each round stays on 2 models and 2 lenses: Spec on the first model, Standards on the second.
- When a round has no finding, the lead starts the pre-ship panel on the same `HEAD`. It is one reviewer, lens Spec, for each listed model that did not review this `HEAD`, all in parallel.
- The guard rule `review-models` denies `harness run ship` until every listed model of one host list has a record on `HEAD`. The floor of 2 models stays.
- The ship transition carries no host. The records pick the host: the rule uses the host list with the most recorded models.
- The condition of 400 lines or a named risk goes. The panel replaces the third reviewer.
- Codex stays as it is: it sends no model, and a Codex repository turns the rule off.

## Consequences

- The panel adds one sequential review step before ship.
- With the default list, opus reviews each final `HEAD`. The cost on every change rises toward side A.
- A rework moves `HEAD`, so each round starts on the first 2 models again.
- A list of 2 models needs no panel step.
- A Codex repository still sets `guard.disabled: [review-models]`.
