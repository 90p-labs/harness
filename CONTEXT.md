# Context

These terms have one meaning in the code, the docs and the plans.

## Run terms

- **Run**: one pass of the loop for one task in one worktree. It has an id, a phase, two round counters and a reason.
- **Phase**: the state of a run. The phases are `planning`, `awaiting_approval`, `working`, `reviewing`, `shipping`, `releasing`, `observing`, `blocked`, `done` and `abandoned`. A new run has the phase none.
- **Event**: a named request to change the phase, such as `start` or `submit-plan`.
- **Guard**: a named pure function of the run and the policy. It decides which row of the transition table applies.
- **Transition**: one table row `(from, event, guard) -> to`. Only one row applies to a phase and an event.
- **Policy**: the settings that guards read, such as the approval need, the round limits and the go-live switch.
- **Review round**: the count of reworks with a real open blocker or major finding. A rework with only minor findings keeps it. Only a regress resets it.
- **Live round**: the count of regressions after a release. Each live round replans the run.
- **Open question**: a decision of the plan that the code and the docs cannot make. It sits under `## Open questions` until the user answers it.
- **Grill**: the rounds of questions that turn each open question into a decision of the user.
- **Frontier**: the open questions whose parent decisions are made. One round asks the whole frontier.
- **Approval record**: a row that holds the run id and the SHA-256 of `plan.md` when the user picks `Approve` on the `Plan` question. `run approve` needs it, unless `--by` names a model.
- **Role**: a job of a subagent, such as `explorer`, `worker`, `reviewer` or `advisor`. `.harness.yaml` gives each role the models that it may use on each host.
- **Review lens**: the question of one reviewer. `Spec` checks the diff against `plan.md`. `Standards` checks it against the repository rules.
- **Review record**: a row that holds the repo, the `HEAD`, the patch id, the files read and the model of one `reviewer` agent. The hook writes it when the agent stops in `reviewing`. `run ship` needs a counted record of every listed `reviewer` model of the host. A record counts when it read a changed file and matches the `HEAD`, the patch id or the run.
- **Review finding**: one row of `findings.md` with an ID, a severity, a verdict and a status. Only a real open row counts for a rework or a ship.
- **Patch id**: the `git patch-id --verbatim` of the diff from the merge base to `HEAD`. A clean rebase keeps it, so the reviews stay valid. A change of whitespace only changes it.
- **Common list**: the findings that one `reviewer` agent for each listed model returns in one pass on the same `HEAD`. The lead merges them and removes duplicates.
- **Verifier**: one agent per review round that checks every finding and returns one line per finding, `real` or `not real`. It never edits a file and writes no review record. On a light panel no verifier runs.
- **Re-review**: one `reviewer` agent that reads all the fixup diffs of a round and answers `closed` or `open` for each. It runs in `working` and writes no review record.
- **Final panel**: the models that review the final `HEAD` after a rework. It is every listed `reviewer` model, or fewer after a small fixup. On a light panel it is the first listed model. `harness review panel` prints it.
- **Docs-only waiver**: a plan with only `docs` commits and no open question skips the approval on `submit-plan`. A docs-only amend in `working` skips it too, when the run has no recorded approval.
- **Kept-structure amend**: an `amend` that skips the approval. It starts in `working` with no live round, and the plan keeps its structure against the approved plan. It may change the tree fences, the test paths of an `Owned files` line, and the prose of Goal, Decisions, `Test first` and `DoD`. Free text keeps the `Track:` line, the `Cause:` line, the test names and the `Seam:` names of `Test first`, and the `DoD` numbers. A task approved with no seam may gain seams. A sketch link anywhere counts as structure.
- **Review risk**: a line of `## Review risks` that names a risk. It points to a test name or to a decision.
- **Seam**: the public interface where a test observes the behaviour of a task. The `Test first` line names it with the label `Seam:`.
- **Evidence baseline**: the table of existing behaviour that the explorer builds when a change modifies, replaces or removes behaviour that is not fully pinned. Each item has one class: `intended`, `bug`, `accidental` or `ambiguous`. Otherwise the explorer writes the line `baseline: not needed`. It differs from the lint baseline of `harness lint baseline`.
- **Stop rule**: the function `DecideStop` in `internal/run/domain/stop.go`. It blocks the end of a turn in a phase that holds the turn, up to the limit of stop blocks. In `shipping`, it allows the end once the run holds a pull request URL, which `harness run wait --pr <url>` sets.
- **Run evidence**: the stored signs of what slowed a run. They are each review rework, each move to the phase `blocked`, the count of amends and the count of stops that the guard denied. `harness run learn` prints each one as an `evidence:` line. A count of 0 prints no line.
- **Lesson**: a change of the agent environment that cites run evidence and has a home. A lesson with no evidence is dropped.
- **Follow-up**: the `harness run start` prompt that the learn step prints for a lesson, a lint rule or a missing seam. Its entry in `docs/follow-ups.md` keeps it, with one `Cited by` line for each run that met the cause.
- **Reproduction**: the section `## Reproduction` of a bug plan, marked by the line `Cause: bug`. It names one `Criterion` whose proof fails for the reported `Symptom`, or says `Human only`. It holds a `Regression test` and 3 to 5 ranked hypotheses. `harness run plan` refuses the plan until that proof failed on the current files.
- **Threat model**: the section that names the attacker, the input forms and the fail-closed rule. A tree path with a `guard` or `security` folder needs it.

## Pull request terms

- **PR evidence**: the `## Evidence` section of the pull request body. It has a `Before:` line and an `After:` line, each with a proof below it. A proof is a screenshot linked by an http or https URL, or a fence with a run command and its output. It differs from the evidence baseline, which is the explorer table of existing behaviour in a plan. It also differs from the evidence record of a gate, which is a stored result of a gate run.
- **Merge danger**: the last section of the pull request body. It has a `Door:` line and a `Blast radius:` line, each with a value and a reason.
- **Door**: the first line of the merge danger. Its value is `two-way` when a revert of the merge restores the old behaviour, and `one-way` when it does not.
- **Blast radius**: the second line of the merge danger. Its value is `small`, `medium` or `large`. The reason says what breaks and for whom if the change is wrong.

## Plugin terms

- **Plugin**: a piece that subscribes to bus events and returns a verdict. A built-in plugin runs in process. An exec plugin runs as a child process.
- **Envelope**: the JSON message that the bus sends to a plugin. It holds the version, the event, the host, the working directory, the run and the payload.
- **Verdict**: the answer of a plugin. It is `allow`, `deny` or `context`.
- **Event kind**: how the bus treats the verdicts of one event. The kinds are `veto`, `collect`, `context` and `observe`.
- **Flat line**: a shell line of simple commands joined by `;`, `&&`, `||`, `|`, `&` or a newline, with no construct that the guard cannot read. `shell-write-source` expands variables and follows `cd` only on a flat line.

## Setup terms

- **Recipe**: a folder of files that `harness recipe apply` writes into a repository. It has a `recipe.yaml`, which lists the files, the scaffold steps and the next steps. Apply writes only the missing files. The word is not `prepare`, which is the `lint.prepare` plugin event.

## Verification and live terms

- **Gate**: one shell command that proves one property of the repo. Its result is `pass`, `fail` or `vetoed`.
- **Lint pack**: a set of lint rules with ids, an engine, a severity and a scope.
- **Signal**: a measure of production, such as an error rate. A plan names its signals and the breach limit of each one.
- **Watch window**: the time after a deploy in which the harness checks the signals. It is the longest window of the signals of the plan.
- **Regression**: a failed deploy or a breached signal. The run goes back to planning with the findings, or to blocked on the last live round.
