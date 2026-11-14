# Program layers around a run

## Goal

Harness drives one run well, and this program adds the three layers around it.
A map holds the decisions before the plan. A coordinator drives the runs of a program. A routine fires a run with no person.

Each run of this plan is one harness run, started by the owner. ADR 0026 holds the design.

## Runs

### R1 map-format

- Wave: 1
- Depends on: none
- Branch: feat/run-program-layers
- Owned files: `internal/plan/domain/`, `internal/plan/application/`, `internal/plan/ui/cli/`, `cmd/harness/plan_map_test.go`, `docs/adr/`, `CONTEXT.md`, `docs/runs.md`, `DESIGN.md`, `docs/design_test.go`, `docs/programs/`, `docs/programs_test.go`, `skills/harness/`
- Done when: `harness plan check --file` prints the frontier of a clean map.

1. Parse and check a map as a third kind of plan document.
2. Print the map frontier from `harness plan check --file`.
3. Record the design of the three layers in ADR 0026 and the terms in `CONTEXT.md`.
4. Commit the map and the multi-run plan of this program.

### R2 map-page

- Wave: 2
- Depends on: R1
- Branch: feat/map-page
- Owned files: `internal/plan/infrastructure/view/`, `internal/plan/application/query/viewpage/`, `internal/plan/domain/view.go`, `internal/plan/ui/cli/view.go`, `internal/plan/ui/cli/view_test.go`, `cmd/harness/plan_map_test.go`, `docs/runs.md`
- Done when: `harness plan view --file` writes a page for a map, with the frontier marked.

1. Render the sections of a map and one card for each decision ticket, with the content that ticket T13 of the map decides.
2. Mark each ticket of the frontier, and draw the blocked-by links.
3. Remove the refusal of a map from the view query.

### R3 run-frontier

- Wave: 2
- Depends on: R1
- Branch: feat/run-frontier
- Owned files: `internal/run/domain/`, `internal/run/infrastructure/persistence/`, `internal/run/application/query/programstatus/`, `internal/run/application/command/start/`, `internal/plan/domain/program.go`, `internal/plan/domain/program_test.go`, `internal/plan/ui/cli/check.go`, `internal/plan/ui/cli/check_test.go`, `cmd/harness/plan.go`, `cmd/harness/plan_test.go`, `DESIGN.md`
- Done when: `harness plan check --file` prints each run of a multi-run plan that is ready to start.

1. Record the branch of a run when it starts, in the place that ticket T7 of the map decides.
2. Read the runs of the repository by branch, over all worktrees.
3. Print one line for each run whose dependencies are done and that no worktree started.

### R4 slice-guide

- Wave: 2
- Depends on: R1
- Branch: docs/slice-guide
- Owned files: `skills/harness/`
- Done when: the harness skill tells how to cut a multi-run plan into slices.

1. Add a skill reference for the plan of a program, with one slice for each run.
2. Describe a wide refactor as expand, migrate and contract, with one run for each step.
3. Bind the reference to the plan parser with a skill test.

### R5 run-origin

- Wave: 3
- Depends on: R3
- Branch: feat/run-origin
- Owned files: `internal/run/domain/`, `internal/run/infrastructure/persistence/`, `internal/run/application/command/start/`, `internal/shared/infrastructure/config/`, `internal/guard/`, `docs/configuration.md`, `DESIGN.md`
- Done when: `harness run start` refuses a start that is deeper than the nesting cap.

1. Record the origin of a run, which is a person, a program or a routine.
2. Record the depth of a run, and add both caps to the config: the nesting depth and the open runs of a program.
3. Deny a start above a cap with a guard rule, not with a skill line.

### R6 program-worktrees

- Wave: 4
- Depends on: R3 and R5
- Branch: feat/program-worktrees
- Owned files: `internal/program/`, `cmd/harness/program.go`, `cmd/harness/program_test.go`, `cmd/harness/main.go`, `internal/plan/domain/program.go`, `internal/plan/domain/program_test.go`, `docs/runs.md`
- Done when: `harness program start` makes the worktree and the branch of one ready run.

1. Add the optional `Integration` line to the multi-run plan.
2. Add the program module and the command `harness program start`.
3. Make one worktree and one branch for a ready run, from the integration branch.
4. Start the run there with the origin program.

### R7 unattended-policy

- Wave: 4
- Depends on: R5
- Branch: feat/unattended-policy
- Owned files: `internal/guard/`, `internal/run/application/command/transition/`, `internal/run/domain/`, `docs/configuration.md`, `DESIGN.md`
- Done when: a run with the origin routine ends `blocked` where the loop asks a person.

1. End a routine run as `blocked`, with the question as the reason, where the loop asks a person.
2. Keep the rule `pr-no-merge` on for a routine run in every config.
3. Keep the docs-only waiver and the `run.approval` policy as the only two ways to skip the first approval of a plan. Set neither from the agent.

### R8 integration-merge

- Wave: 5
- Depends on: R6
- Branch: feat/integration-merge
- Owned files: `internal/program/`, `cmd/harness/program.go`, `cmd/harness/program_test.go`, `agents/`, `test/agents/`, `docs/runs.md`
- Done when: `harness program merge` merges each done run branch onto the integration branch in wave order.

1. Merge each done run branch in wave order, and stop at the first conflict.
2. Add the merger in the form that ticket T8 of the map decides. It resolves a conflict and never changes the scope of a run.
3. With no agent of its own, the merger uses the models of the worker role, with no new role key.

### R9 routine-config

- Wave: 5
- Depends on: R7
- Branch: feat/routine-config
- Owned files: `internal/routine/`, `internal/shared/infrastructure/config/`, `cmd/harness/routine.go`, `cmd/harness/routine_test.go`, `cmd/harness/main.go`, `docs/configuration.md`, `DESIGN.md`
- Done when: `harness routine fire` starts one run for a routine and refuses a second firing for the same work.

1. Add the `routines` section to the config, with the keys that ticket T9 of the map decides.
2. Add the command `harness routine fire`, which starts at most one run.
3. Record each firing in the shared state database, as the lock and as the count for the caps.

### R10 program-review

- Wave: 6
- Depends on: R8
- Branch: feat/program-review
- Owned files: `internal/program/`, `cmd/harness/program.go`, `cmd/harness/program_test.go`, `skills/harness/`, `docs/runs.md`
- Done when: one review panel reads the diff of the integration branch against the default branch.

1. Add the command `harness program review`, which prints the panel for the integration branch.
2. Send each finding back to the run that owns the file, in the way that ticket T10 of the map decides.
3. Describe the program review in the harness skill.

### R11 github-trigger

- Wave: 6
- Depends on: R9
- Branch: feat/github-trigger
- Owned files: `internal/recipe/`, `docs/unattended.md`
- Done when: a recipe installs a GitHub Actions workflow that fires a routine.

1. Add a recipe with one workflow for a label of an issue, a pull request event and a schedule.
2. Use the token of a GitHub App, because an event of the default token starts no new workflow run. Ticket T11 of the map must be closed first.
3. Document the rate limit of each identity and the cap of each loop.

### R12 ratchet-routine

- Wave: 6
- Depends on: R9
- Branch: feat/ratchet-routine
- Owned files: `internal/lint/`, `internal/routine/`, `cmd/harness/lint.go`, `cmd/harness/lint_test.go`, `DESIGN.md`
- Done when: the routine `ratchet` opens one pull request that removes one entry of the lint baseline.

1. Add the command `harness lint ratchet`, which picks the entry with the fewest findings.
2. Add the routine `ratchet` as an example, fired by a clock.
3. End the firing with no pull request when the baseline is empty.

## Decisions

- The owner starts each run. This plan starts none.
- This plan is written before the map is clear. A run that waits on an open ticket names it, and the owner closes the ticket before the run starts.
- The words run and task keep their harness meaning. The outer layer uses routine, trigger and firing.
- An unattended run always ends at a pull request that a person merges.
- A cap is a config key that a guard reads. No cap is a line of a skill.
- A tracker of tickets and a runner are out of scope. The map is a file, and the git host or a cron fires a routine.
- The map and this plan change between the runs, in a docs change of their own. So no later run owns `docs/programs/`.
