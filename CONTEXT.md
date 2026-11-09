# Context

These terms have one meaning in the code, the docs and the plans.

## Run terms

- **Run**: one pass of the loop for one task in one worktree. It has an id, a phase, two round counters and a reason.
- **Phase**: the state of a run. The phases are `planning`, `awaiting_approval`, `working`, `reviewing`, `shipping`, `releasing`, `observing`, `blocked`, `done` and `abandoned`. A new run has the phase none.
- **Event**: a named request to change the phase, such as `start` or `submit-plan`.
- **Guard**: a named pure function of the run and the policy. It decides which row of the transition table applies.
- **Transition**: one table row `(from, event, guard) -> to`. Only one row applies to a phase and an event.
- **Policy**: the settings that guards read, such as the approval need, the round limits and the go-live switch.
- **Review round**: the count of rework requests since the last plan approval.
- **Live round**: the count of regressions after a release. Each live round replans the run.
- **Open question**: a decision of the plan that the code and the docs cannot make. It sits under `## Open questions` until the user answers it.
- **Grill**: the rounds of questions that turn each open question into a decision of the user.
- **Frontier**: the open questions whose parent decisions are made. One round asks the whole frontier.
- **Approval record**: a row that holds the run id and the SHA-256 of `plan.md` when the user picks `Approve` on the `Plan` question. `run approve` needs it, unless `--by` names a model.
- **Role**: a job of a subagent, such as `explorer`, `worker`, `reviewer` or `advisor`. `.harness.yaml` gives each role the models that it may use on each host.
- **Review lens**: the question of one reviewer. `Spec` checks the diff against `plan.md`. `Standards` checks it against the repository rules.
- **Review record**: a row that holds the repo, the `HEAD` and the model of one dispatched `reviewer` agent. Only a `reviewer` agent call in `reviewing` writes one. `run ship` needs a record of every listed `reviewer` model of the host on `HEAD`.
- **Common list**: the findings that one `reviewer` agent for each listed model returns in one pass on the same `HEAD`. The lead merges them and removes duplicates.
- **Verifier**: an agent that checks one finding and answers `real` or `not real`. It never edits a file and writes no review record.
- **Re-review**: a `reviewer` agent that reads one fixup diff and answers `closed` or `open`. It runs in `working` and writes no review record.
- **Final panel**: the common list on the final `HEAD` after a rework, with every listed `reviewer` model.
- **Docs-only waiver**: a plan with only `docs` commits and no open question skips the approval on `submit-plan`.

## Plugin terms

- **Plugin**: a piece that subscribes to bus events and returns a verdict. A built-in plugin runs in process. An exec plugin runs as a child process.
- **Envelope**: the JSON message that the bus sends to a plugin. It holds the version, the event, the host, the working directory, the run and the payload.
- **Verdict**: the answer of a plugin. It is `allow`, `deny` or `context`.
- **Event kind**: how the bus treats the verdicts of one event. The kinds are `veto`, `collect`, `context` and `observe`.
- **Flat line**: a shell line of simple commands joined by `;`, `&&`, `||`, `|`, `&` or a newline, with no construct that the guard cannot read. `shell-write-source` expands variables and follows `cd` only on a flat line.

## Verification and live terms

- **Gate**: one shell command that proves one property of the repo. Its result is `pass`, `fail` or `vetoed`.
- **Lint pack**: a set of lint rules with ids, an engine, a severity and a scope.
- **Signal**: a measure of production, such as an error rate. A plan names its signals and the breach limit of each one.
- **Watch window**: the time after a deploy in which the harness checks the signals. It is the longest window of the signals of the plan.
- **Regression**: a failed deploy or a breached signal. The run goes back to planning with the findings, or to blocked on the last live round.
