# 0016 Review with two models

## Status

Accepted, 2026-11-09. ADR 0017 replaces its third-reviewer rule. Ship now needs every listed reviewer model of the host, and the floor of 2 models stays.

## Context

One reviewer on one model misses errors that another model finds. Two studies give the evidence:

- arXiv 2610.01471v2 (2026-10-06): one fresh-context review plus one review by a different model found 56.7% of planted errors. Two same-model reviews found 42.7% (p = .006).
- The same paper: a same-model fresh-context review was best on code (F1 40.7%). A lightweight reviewer alone was below self-review (24.0% against 27.1%). A second review in the same session dropped F1 to 21.7%.
- arXiv 2606.15689 (2026-04-09): models largely detect the same bugs, and a second model adds its false positives.

So a second model alone adds little. The lenses must differ, not only the models.

## Decision

- Each change gets at least 2 reviews, each in a fresh context, on 2 different models. One review has the lens Spec: the diff against `plan.md`. The other has the lens Standards: the diff against the repository rules, then edge cases and leaks.
- A third reviewer joins on the last model of the role for a diff of more than 400 lines, or a plan that names a state, concurrency or security risk.
- The `roles` key maps each host to a list of models. The guard rule `agent-model-routing` allows any listed model and denies another model or no model.
- A new role `advisor` reads the plan before `harness run plan`. It never counts as a reviewer.
- After each Agent call of the `reviewer` role, a hook records the model for the current `HEAD`. The guard rule `review-models` denies `harness run ship` when fewer than 2 distinct models reviewed `HEAD`.
- The lead drives the work: one `worker` agent for each task, on the model of the task.

## Consequences

- The record proves a dispatch, not a finished review. A background agent sends its event at dispatch. The skill tells the lead to count a gap as no review.
- Codex sends no Agent call with a model. A codex repository needs `guard.disabled: [review-models]`.
- A rework moves `HEAD`, so each round needs new reviews.
- The cost of a review changes. The page `docs/bench/review-models.md` gives the measured time, tokens and findings.
