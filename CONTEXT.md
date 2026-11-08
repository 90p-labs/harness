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

## Plugin terms

- **Plugin**: a piece that subscribes to bus events and returns a verdict. A built-in plugin runs in process. An exec plugin runs as a child process.
- **Envelope**: the JSON message that the bus sends to a plugin. It holds the version, the event, the host, the working directory, the run and the payload.
- **Verdict**: the answer of a plugin. It is `allow`, `deny` or `context`.
- **Event kind**: how the bus treats the verdicts of one event. The kinds are `veto`, `collect`, `context` and `observe`.

## Verification and live terms

- **Gate**: one shell command that proves one property of the repo. Its result is `pass`, `fail` or `vetoed`.
- **Lint pack**: a set of lint rules with ids, an engine, a severity and a scope.
- **Signal**: a measure of production, such as an error rate. A plan names its signals and the breach limit of each one.
- **Watch window**: the time after a deploy in which the harness checks the signals. It is the longest window of the signals of the plan.
- **Regression**: a failed deploy or a breached signal. The run goes back to planning with the findings, or to blocked on the last live round.
