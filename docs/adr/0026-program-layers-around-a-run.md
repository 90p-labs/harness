# 0026 Program layers around a run

## Status

Accepted, 2026-11-14. It adds the map as a third kind of plan document, built in this run. It adds no phase and no event, so the state machine and the plugin events do not change.

The later runs of the program build the other parts. The multi-run plan is `docs/programs/run-program-layers/plan.md`, and the map is `docs/programs/run-program-layers/map.md`.

## Context

- Harness drives one run well. A multi-run plan has waves, dependencies, branches and owned files. `harness plan check --file` and `harness plan view --file` read it.
- Three things are missing. Nothing decides what goes into the multi-run plan. Nothing executes its graph. Nothing starts a run with no person.
- The words run and task already have a harness meaning, so the outer layers need other words.
- The idea adapts skills of Matt Pocock (MIT license), at commit `5863806`. The skills are wayfinder, to-tickets, implement, implement-spec and the in-progress software-factory.
- The sources are <https://github.com/mattpocock/skills/blob/main/skills/engineering/wayfinder/SKILL.md>, <https://github.com/mattpocock/skills/blob/main/skills/engineering/to-tickets/SKILL.md>, <https://github.com/mattpocock/skills/blob/main/skills/engineering/implement/SKILL.md>, <https://github.com/mattpocock/skills/blob/main/skills/engineering/implement-spec/SKILL.md> and <https://github.com/mattpocock/skills/blob/main/skills/in-progress/software-factory/SKILL.md>.
- The model of the layers comes from those skills. The text here is new. The survey files of software-factory are dated 2026-10, so each product fact was checked again.

## Decision

1. Words. Run and task keep their meaning. Layer 1 uses map, decision ticket and map frontier. Layer 2 uses program, coordinator, slice, run frontier, integration branch, merger and program review. Layer 3 uses routine, trigger and firing. The grill term Frontier stays as it is.
2. Layer 1 is the map. A map is a markdown file for a large effort, written before the multi-run plan. Its sections are Destination, optional Notes, Decisions so far, Not yet specified, Out of scope and Tickets.
3. A decision ticket holds one question. Its type is research, prototype, grilling or work. The source says task, but a task is already a part of a plan.
4. A ticket is open or closed. It can be blocked by other tickets. A session can claim it with a line in the file.
5. The map frontier is each ticket that is open, has every blocker closed and has no claim. The rule is to plan and not to do. A ticket ends with an answer, never with a change of the product, and a prototype is thrown away.
6. A map is clear when no ticket is open and `Not yet specified` holds only `- none`. Then the multi-run plan is final. A plan written before that names the open ticket in each run that waits on it. The owner closes that ticket before the run starts.
7. `harness plan check --file` reads a map as a third kind of plan document and prints the frontier. A map needs no run. `harness run plan` and, for now, `harness plan view` refuse a map.
8. Layer 1 and the multi-run plan: the map comes before it. The closed tickets are the decisions that the multi-run plan stands on. The map and the multi-run plan of a program live in one folder under `docs/programs/`.
9. Layer 2 is the program. A program is the execution of one multi-run plan. A slice is the work of one run of that plan. It is small enough for one fresh context, which means a single-run plan of 6 tasks at most.
10. The coordinator is the session that drives a program. It starts runs and writes no code. The run frontier is each run whose dependencies are done and that no worktree started. Each run gets its own worktree and branch.
11. The merger merges each done run branch onto one integration branch, in wave order. It resolves conflicts without a change of the scope of a run. The program review is one review panel. It reads the diff of the integration branch against the default branch.
12. A wide refactor is three steps, expand, migrate, contract, with one run for each step.
13. Layer 2 and the multi-run plan: the multi-run plan is the graph. The program adds no second graph file. The state of the graph comes from the run records.
14. Layer 3 is the routine. A routine is a named entry of the config that starts a run with no person. A trigger is what fires it: a ticket (a label on an issue), a pull request event or a clock.
15. A firing is one start of a routine, and it starts one run at most. An unattended run never waits for a person. Where the loop asks a person, the run ends `blocked` with the question as the reason.
16. Brakes. Three brakes replace the person inside the loop. They are the gates (automated checks), the review panel (automated review) and a person who merges the pull request (human review).
17. An unattended run never merges, so the rule `pr-no-merge` stays on for it in every config. No guard rule is weaker for an unattended run. A plan skips its first approval only by the docs-only waiver or the `run.approval` policy. The owner sets the policy in the config or in the environment of the trigger. The agent never sets it.
18. Caps. Every loop has a cap. The caps are:
   - the review rounds, which exist today.
   - the nesting depth of runs.
   - the open runs of a program.
   - the firings of a routine in a window.
19. A cap is a config key that a guard reads. Recursion and nesting limits belong to harness, not to a skill.
20. GitHub identity, checked on docs.github.com on 2026-11-14. An event that the default `GITHUB_TOKEN` causes creates no new workflow run, except `workflow_dispatch` and `repository_dispatch`. So a chained run needs its own identity. It is the installation token of a GitHub App, or a personal access token.
21. Rate limits count for each identity. With `GITHUB_TOKEN` the limit is 1,000 requests an hour for each repository. A GitHub App installation gets at least 5,000 an hour. A user gets 5,000 an hour. These facts change, so the date above is part of the record.
22. Layer 3 and the multi-run plan: a routine starts one run, not a program. A routine can fire a run of a program only as a later step, outside this program.
23. The example routine is the ratchet. A clock fires it, and one firing opens one pull request that removes one entry of the lint baseline.
24. Out of scope:
   - a ticket tracker with a server, because the map is a file.
   - a runner or a scheduler of its own, because the git host or a cron fires a routine.
   - an orchestrator that starts the runs of a program with no owner.
   - a merge by an unattended run.

## Consequences

- A large effort has one file for its decisions before a plan exists.
- The first slice ships only the map format. The rest is a multi-run plan that the owner starts run by run.
- A claim is a line in a file. Two sessions on two machines can claim one ticket until they pull. A claim store is an open ticket of the map.
- `harness plan view` has no page for a map until a later run.
- An unattended run always ends at a pull request, so a person is still in the path of each change.
- No guard gets weaker, so the threat model of ADR 0019 does not change.
- The GitHub facts need a new check when the work on the trigger starts.
