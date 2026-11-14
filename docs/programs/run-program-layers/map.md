# Map: Program layers around a run

## Destination

Harness plans a large effort, executes the graph of its runs, and starts a run with no person.
A map holds the decisions before the plan. A coordinator drives the runs of a program. A routine fires a run from a trigger.

## Notes

ADR 0026 holds the design. The multi-run plan is `plan.md` in this folder.
This map plans and does not do. A ticket ends with an answer, and the work goes into a run of the plan.

## Decisions so far

- T1: the words run and task keep their harness meaning, and the outer layer uses program, slice, routine, trigger and firing.
- T2: the map is a third kind of plan document, read by `harness plan check --file`.
- T3: the first slice is layer 1, the map format and its check.
- T4: the brakes are the gates, the review panel and a person who merges the pull request.
- T5: a chained workflow uses the token of a GitHub App or a personal token, and the default token starts no new workflow run.
- T6: a cap is a config key that a guard reads, and no cap is a line of a skill.

## Not yet specified

- The page of a map in `harness plan view`.
- The place of the branch and of the origin in the run record.
- The merger: a new agent, or the worker with another brief.
- The config keys of a routine and of its caps.
- The way that a program review sends a finding back to a run.
- A claim store for tickets, beyond a line in the file.

## Out of scope

- A tracker of tickets with a server. The map is a file.
- A runner or a scheduler of its own. The git host or a cron fires a routine.
- A merge by an unattended run.
- An orchestrator that starts the runs of the program with no owner.

## Tickets

### T1 run-word

- Type: grilling
- Blocked by: none
- Status: closed
- Question: Which words name the outer layers, so that run and task keep their meaning?
- Answer: Program and slice for layer 2, and routine, trigger and firing for layer 3.

### T2 map-home

- Type: research
- Blocked by: none
- Status: closed
- Question: Where does the map live, and which command reads it?
- Answer: It is a markdown file, and `harness plan check --file` reads it as a third kind of plan document.

### T3 first-slice

- Type: grilling
- Blocked by: T1 and T2
- Status: closed
- Question: Which slice has the most value in six tasks?
- Answer: Layer 1, because the other layers need its decisions and the plan parser is its only dependency.

### T4 brakes

- Type: grilling
- Blocked by: T1
- Status: closed
- Question: Which brakes replace the person in an unattended run?
- Answer: The gates, the review panel, and a person who merges the pull request.

### T5 github-identity

- Type: research
- Blocked by: none
- Status: closed
- Question: Which identity lets a workflow start another workflow on GitHub?
- Answer: The token of a GitHub App or a personal token, as the GitHub docs said on 2026-11-14.

### T6 cap-owner

- Type: grilling
- Blocked by: T4
- Status: closed
- Question: Where does a limit on nesting and on loops live?
- Answer: In the config, as a key that a guard reads.

### T7 run-branch

- Type: research
- Blocked by: none
- Status: open
- Question: Where does the run record keep the branch and the origin of a run?

### T8 merger-agent

- Type: prototype
- Blocked by: T7
- Status: open
- Question: Does the merger need an agent of its own, or is it the worker with another brief?

### T9 routine-keys

- Type: grilling
- Blocked by: T6
- Status: open
- Question: Which keys does a routine have in the config?

### T10 finding-route

- Type: grilling
- Blocked by: T8
- Status: open
- Question: How does a program review send a finding back to the run that owns the file?

### T11 app-access

- Type: work
- Blocked by: T5
- Status: open
- Question: Who creates the GitHub App of the organisation and installs it on the repository?

### T12 claim-store

- Type: grilling
- Blocked by: none
- Status: open
- Question: Does a claim need a store beyond a line in the file, so that two sessions cannot claim one ticket?

### T13 map-page

- Type: grilling
- Blocked by: none
- Status: open
- Question: What does the page of a map show in `harness plan view`, beyond the cards and the frontier?
