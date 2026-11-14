# Configuration

This page explains each key of `.harness.yaml` and how to configure it. Each key gives its type, its default, its env variable and its effect. The [full configuration](configuration-full.md) page shows every key and one example for each case.

## Decision tree

```
What do you need?
├── set up a new repository → "A new repository" in "How to configure"
├── a key does not take effect → "Precedence"
├── change the checks that run before a commit
│   ├── a check command → `gates`
│   ├── a step before the checks (install, generate) → `setup`
│   └── run more checks at the same time → `verify`
├── change how a run moves
│   ├── no human approval of the plan → `run`
│   ├── more rework rounds → `run`
│   └── follow the change to production → `run` and `live`
├── a guard deny blocks a legal action → `guard`
├── an agent call runs on the wrong model → `roles`
├── the architecture rules → `lint`
├── the deploy plugins (helm, promtool) → `deploy`
├── your own command on an event → `plugins`
├── the tools that the Claude hooks fire for → `hooks`
├── the push target of the guard → `git`
├── the state database → `state`
└── see every key and one example for each case → docs/configuration-full.md
```

## Where the file lives

- The file is `.harness.yaml` at the git top level of the repository.
- The root flag `--config <path>` reads another file. A missing `--config` file is an error.
- A missing `.harness.yaml` gives the defaults. Without the file, the hooks do nothing.
- An unknown key is an error that names the key. A typo does not pass without a sign.
- `harness config` prints the effective `state`, `run`, `plugins`, `roles`, `guard` and `git`. It does not print the other keys.

## Precedence

The harness reads each key from the first source that sets it:

1. A flag. Only `state.dsn` has one: `--state-dsn`.
2. An env variable. The name is `HARNESS_`, then the key in capitals, with `_` for each dot: `HARNESS_RUN_MAX_REVIEW_ROUNDS` sets `run.max_review_rounds`.
3. The file `.harness.yaml`.
4. The default.

The env variable has limits:

- Only a key with a default reads its env variable. The Env column below gives `none` for the other keys.
- A key with no default, such as `lint.packs`, `gates` or `setup`, ignores its env variable. You get no error.
- A list takes comma separated values: `HARNESS_GUARD_DISABLED=pr-no-merge,pr-draft-only`. A space does not separate two values.
- `HARNESS_PLUGINS` is an error. Set the plugins in the file.

A duration, such as a gate or plugin `timeout`, is a string with a unit: `30s`, `750ms` or `2m`. The number `30` is an error.

## Telemetry env variables

Telemetry is not a key of `.harness.yaml`. Set the endpoint in the env. Under Claude Code, use the `HARNESS_` names, because Claude Code removes every `OTEL_*` variable.

- `HARNESS_OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`: the traces endpoint.
- `HARNESS_OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`: the metrics endpoint.
- `HARNESS_OTEL_EXPORTER_OTLP_ENDPOINT`: one endpoint for both signals.
- `HARNESS_OTEL_TRACE_DETAIL`: the detail level of the `plugin.call` spans. `compact` is the default. `full` keeps a span for each plugin call.

For one signal, the harness reads the first env variable that is set, in this order:

1. `HARNESS_OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`
2. `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`
3. `HARNESS_OTEL_EXPORTER_OTLP_ENDPOINT`
4. `OTEL_EXPORTER_OTLP_ENDPOINT`

Metrics use the same order with `METRICS`. The harness joins `/v1/traces` or `/v1/metrics` to the path of a general endpoint. It uses a signal endpoint as it is. For example:

```sh
HARNESS_OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://localhost:4318/v1/traces
```

### The trace of a run

Each run has one trace. The harness derives the trace ID and the ID of the root span from the run ID.

- Each harness process of an active run is a child of the root span `run`. This includes hook calls, verify, plan check and run transitions.
- A `TRACEPARENT` in the same trace stays the parent. A gate child process stays under its gate span.
- A `TRACEPARENT` in another trace is replaced by the root span of the run.
- A process with no active run starts its own trace. The exception is `harness run start`. Its `run.transition` span joins the trace of the new run.
- The harness exports the root span `run` when the run reaches the phase done or abandoned. For example, `harness run done`, `harness run abandon` or the live check does it. The span starts at the start time of the run and ends now. It has `harness.run.id`, `harness.worktree` and `harness.run.phase`.
- `agent.turn`: one span for each turn of an agent session in an active run. It starts at a prompt (`UserPromptSubmit`). It ends at the next stop that the harness allows, or at the next prompt. It has `harness.session.id` and `harness.turn.end` (`stop` or `prompt`).
- `agent.subagent`: one span for each subagent. It starts at `SubagentStart` and ends at `SubagentStop`. It has `harness.session.id`, `harness.agent.id` and `harness.agent.type`.
- Both agent spans are children of the root span `run`. The harness exports each span at its closing hook. It uses the start time that it stores in the state database.

A hook cannot see model time or a server tool call. A gap between hook spans sits inside a turn or subagent span. The agent spans have these limits:

- A denied stop keeps the turn open.
- Queued task notifications come as prompts. A prompt closes the open turn and opens a new one.
- A user interrupt fires no stop. The turn then ends at the next prompt and holds the idle time.
- With no active run, the harness stores no mark. The first turn of a run has no span.
- A turn or subagent with no closing hook has no span.
- The `tool.call` span covers a tool call of Claude Code only.

### Spans that answer an audit

- `plugin.call`: with `compact`, the harness keeps the span only for a deny or an error. With `full`, it keeps every span.
- `hook.handle`: has `harness.tool` on a tool hook. On a deny, it has `harness.rule`, the names of the denying plugins, such as `guard.<rule id>`.
- `run.transition`: has `harness.review.round`. A `request-rework` also has `harness.rework.class` and `harness.rework.severity`.
- `tool.call`: one span for each tool call. It starts at `PreToolUse`. It ends at `PostToolUse`, `PostToolUseFailure` or `SessionEnd`. It has `harness.tool`, `harness.tool.kind`, `harness.tool.outcome` (`succeeded`, `failed` or `interrupted`) and `harness.session.id`.
- `run.report`: one span for each call of `harness run report`.
- `run.learn`: one span for each call of `harness run learn`.
- No span carries command text, tool input, a response or an error text from a tool.

### Audit a run

`harness run report [<run id>]` prints the rework rounds, the amends, the stop blocks and the rework classes. It also prints the minutes in each phase and the trace ID. Use the trace ID to find the trace in Jaeger.

`harness run learn [<run id>]` prints the convention candidates of the run, one `candidate:` line for each. A candidate is a real Standards finding, or a real finding that a `gate-checkable` rework names. Then it prints one `evidence:` line for each rework and each block, in log order, with the IDs `rework-<n>` and `block-<n>`. The `amends` and `stop-blocks` lines come last, and each one appears only when its count is above 0. A `next:` line ends the output. The command is read-only.

A bug plan with the line `Regression test: none: <reason>` adds a first candidate with the source `no-seam`.

To count the denies, the reworks, the amends, the stop blocks and the tool outcomes, read the trace from Jaeger. Use the jq program `docs/audit-trace.jq`:

```sh
curl -sf localhost:16686/api/v3/traces/<trace id> | jq -e -f docs/audit-trace.jq
```

The program prints nothing and `jq -e` fails when the trace has no `run.transition` span.

The root span appears only after the run reaches done or abandoned. Until then, the trace in the backend has no root span. If that export fails, the trace has no root span.

## `state`

The run state database. It holds the runs, the plan approvals, the gate results, the commit checks and the agent marks (the open turns and subagents of a run).

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `dsn` | string | `sqlite:.harness/state.db` | `HARNESS_STATE_DSN` | The database. The only engine is `sqlite`. Must not be empty. Flag: `--state-dsn` |

A relative path starts at the git common folder (`.git/`), so every worktree of a clone shares one state. The default file is `.git/.harness/state.db`.

```yaml
state:
  dsn: sqlite:/var/lib/harness/state.db
```

## `run`

The policy of the run loop. `harness run` reads it on each transition.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `approval` | bool | `true` | `HARNESS_RUN_APPROVAL` | `true` stops each plan at `awaiting_approval` until a human picks Approve. `false` moves the plan to `working`. A plan after a production regression always needs approval |
| `go_live` | bool | `false` | `HARNESS_RUN_GO_LIVE` | `true` adds the phases `releasing` and `observing` after the merge. See [go-live](go-live.md) |
| `max_review_rounds` | int, 1 or more | `3` | `HARNESS_RUN_MAX_REVIEW_ROUNDS` | The review rounds of a run. A rework request in the last round moves the run to `blocked` |
| `max_live_rounds` | int, 1 or more | `3` | `HARNESS_RUN_MAX_LIVE_ROUNDS` | The production regressions that send the run back to `planning`. The last one moves the run to `blocked` |
| `strict_minor` | bool | `false` | `HARNESS_RUN_STRICT_MINOR` | `false` counts a review round only for a real open blocker or major finding. A minor finding moves the run to `working` and keeps the round. `true` counts a minor finding like a major one |
| `panel_fixup_lines` | int, 1 or more | `80` | `HARNESS_RUN_PANEL_FIXUP_LINES` | The largest fixup, in added plus deleted lines, that keeps the small final review panel after a rework. A larger fixup needs every listed reviewer model |
| `max_stop_blocks` | int, 1 or more | `3` | `HARNESS_RUN_MAX_STOP_BLOCKS` | How many times in a row the guard refuses the end of an agent turn in a phase that holds the turn |

A plan with only `docs` commits and no open question skips the approval, also with `approval: true`.

The guard also allows one stop for each phase change when `handoff.md` is ready. The lead writes `handoff.md` in the run folder after a transition. The file has the lines `Phase: <phase>` and `Step: <n>`. It has these 4 sections:

- `## Next`
- `## Open findings`
- `## Unreviewed commits`
- `## Decisions`

One refusal lists every problem. It names the Phase line, the Step line, and each missing or empty section. Each section has at least one `- ` line.

The file has at most 4096 bytes, and `harness run status` prints `handoff: ready (step <n>)` after the stop. The run stores the step in the column `handoff_step`. A second stop at the same step blocks, as `max_stop_blocks` says. `handoff.md` is a checkpoint. The `handoff:` line ends with `then continue`: the lead writes the file, then continues with the `next:` step. No host resumes a run by itself: a new session reads `harness run status` and `handoff.md`.

```yaml
run:
  approval: true
  max_review_rounds: 5
```

## `hooks`

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `matched` | bool | `true` | `HARNESS_HOOKS_MATCHED` | `true` gives the Claude `PreToolUse` and `PostToolUse` hooks a matcher. They fire only for the tools that a guard rule reads. These are `Bash`, `Edit`, `MultiEdit`, `Write`, `Read`, `Grep`, `Glob`, `Agent` and `AskUserQuestion`, also with a host prefix such as `mcp__server__life_Bash`. `false` writes the hooks with no matcher, so they fire for every tool. The plugin file `hooks/hooks.json` is always matched. Codex hooks have no matcher |

A matched hook never sees another tool, so an exec plugin on `tool.before` or `tool.after` does not run for it. To keep such a plugin working, set `matched: false` and run `harness host install` again.

```yaml
hooks:
  matched: false
```

## `plugins`

Your own commands on the event bus. The harness writes the event envelope as JSON on stdin and reads one verdict from stdout. The [plugins page](plugins.md) gives the verdict format and an example for each plugin kind.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `name` | string | required | none | The plugin name. Unique in the list. The bus calls the plugins of one event in name order |
| `command` | list of strings | required | none | The program and its arguments. The harness runs it with no shell |
| `events` | list of strings | required | none | One event name or more, from the table below. An unknown name is an error |
| `timeout` | duration | `5s` | none | The longest run of one call. `0` gives `5s`. A negative value is an error |
| `transition` | list of strings | none | none | Optional. Only for the events `run.transition.before` and `run.transition.after`; any other event with it is an error. The plugin runs only for these transitions: `start`, `submit-plan`, `approve-plan`, `amend`, `request-rework`, `rework-minor`, `submit-for-review`, `approve-review`, `complete`, `deploy`, `close`, `regress`, `block`, `resume`, `abandon`. A value in one key is an OR. The three keys together are an AND |
| `from` | list of strings | none | none | Optional. Same rules as `transition`. The plugin runs only when the run leaves one of these phases: `planning`, `awaiting_approval`, `working`, `reviewing`, `shipping`, `releasing`, `observing`, `blocked`, `done`, `abandoned`. The transition `start` has no phase before it, so `from` never matches it |
| `to` | list of strings | none | none | Optional. Same rules as `transition`. The plugin runs only when the run enters one of these phases: `planning`, `awaiting_approval`, `working`, `reviewing`, `shipping`, `releasing`, `observing`, `blocked`, `done`, `abandoned` |

The events:

| Event | Kind | Source |
|---|---|---|
| `session.started` | context | host hook |
| `prompt.submitted` | veto | host hook |
| `tool.before` | veto | host hook |
| `tool.after` | context | host hook |
| `turn.stopping` | veto | host hook |
| `subagent.started` | context | host hook |
| `subagent.stopped` | observe | host hook |
| `run.transition.before` | veto | run module |
| `run.transition.after` | observe | run module |
| `plan.submitted` | collect | plan module |
| `gate.before` | veto | gate module |
| `gate.after` | observe | gate module |
| `signals.check` | collect | live module |
| `gates.detect` | collect | `harness gates init` |
| `chart.render` | collect | lint pack `observability` |
| `lint.check` | collect | lint module |
| `lint.prepare` | collect | lint module |

A `veto` deny stops the action. A `collect` event joins the answers of all plugins. A `context` event keeps only the context text. An `observe` event drops each verdict. [DESIGN.md](../DESIGN.md#plugin-events) gives the payload of each event.

```yaml
plugins:
  - name: signals
    command: [scripts/signals.sh]
    events: [signals.check]
    timeout: 30s
```

### Filter a transition

The keys `transition`, `from` and `to` pick the run transitions that a plugin sees. The transition names are `start`, `submit-plan`, `approve-plan`, `amend`, `request-rework`, `rework-minor`, `submit-for-review`, `approve-review`, `complete`, `deploy`, `close`, `regress`, `block`, `resume` and `abandon`. The phase names are `planning`, `awaiting_approval`, `working`, `reviewing`, `shipping`, `releasing`, `observing`, `blocked`, `done` and `abandoned`.

- Values in one key are an OR. The keys together are an AND.
- A plugin with no filter sees all transitions.
- The bus matches the filter before it starts the process. A skipped plugin never runs.
- The transition `start` has an empty `from`, so a `from` filter never matches it.

The payload of `run.transition.before` is `{"event","from","to","reason"}`. In the `run.transition.before` payload `to` is the phase that the transition would reach. When the save fails, the run did not reach it.

The kind of `run.transition.before` is veto: a deny stops the transition. The kind of `run.transition.after` is observe: the bus drops the verdict and ignores a plugin error without a message. On `run.transition.after`, the envelope `run.phase` is the source phase. Read `payload.to` for the new phase.

A pre-hook that checks each deploy:

```yaml
plugins:
  - name: predeploy-check
    command: [scripts/predeploy.sh]
    events: [run.transition.before]
    transition: [deploy]
```

A post-hook that tells the team when a plan waits for approval:

```yaml
plugins:
  - name: notify-approval
    command: [scripts/notify.sh]
    events: [run.transition.after]
    to: [awaiting_approval]
```

## `roles`

The models of each agent role on each host. A host maps to a list of models, and a single model still loads as a list of one. The guard rule `agent-model-routing` denies an agent call whose model is not in the list of its role. A role with no model for the host is not checked.

The rule `review-models` needs a counted review of every model of the final panel. It needs at least 2 models, or 1 model on a light panel. The panel is every reviewer model of the host list, unless a small or light panel applies. A review counts when it read a changed file and it matches `HEAD`, the patch id, or the current run. The ship transition carries no host, so the rule picks the host list with the most recorded models.

The small panel is the first 2 reviewer models plus the models in the `Model` cell of the findings that the last `request-rework` reason names. It applies only when the last rework event is a `request-rework` and the fixup is at most `run.panel_fixup_lines` lines. Every named blocker or major finding needs a listed model in its `Model` cell. The fixup is the added plus deleted lines from the last `HEAD` that every listed model reviewed. Any missing fact gives every listed model. `harness review panel` prints the panel and the reason.

A light panel applies to a light track with no guard, security or hook path. Both `plan.md` and `approved-plan.md` of the run must declare the light track, or an empty `Track:` when the changed tree classifies light. The code tree of the plan and the tree of the changed files must both classify light by the same rule. It needs fewer than 3 folders, no `domain`, `migrations`, `command` or `query` folder, and no `.sql` file. The changed files must also touch no guard, security or hook path.

The approved copy must also match the latest approval record of the run. A run with no approval record never gets a light panel. Examples are a plan with only `docs` commits, which skips approval, and a run with `run.approval: false`.

On a light panel, `harness review panel` prints one model, the first listed, with the reason `light panel: light track with no guard, security or hook path`. A light panel has a floor of 1 model, and the lead writes `self-check.md` with no self-check worker. No verifier runs. The base (`git.default_branch`, or origin HEAD) must name a local or origin branch. A tag or a revision such as `HEAD~1` gives the full panel.

On a full panel, a host that lists a `reviewer` role needs 2 or more models there, or `review-models` in `guard.disabled`. With one listed model, `harness run ship` is denied on a full panel, and the next step names `roles.reviewer`. A light panel ships with one model.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `roles` | map of role to a map of host to a list of models | `lead.claude: [opus]`, `lead.codex: [gpt-6-astra]`, `explorer.claude: [haiku]`, `explorer.codex: [gpt-6-luna]`, `worker.claude: [sonnet, haiku]`, `worker.codex: [gpt-6.1-sol, gpt-6-luna]`, `reviewer.claude: [sonnet, haiku, opus]`, `reviewer.codex: [gpt-6.1-sol, gpt-6.1-terra, gpt-6-astra]`, `advisor.claude: [fable, opus]`, `advisor.codex: [gpt-6-astra]` | `HARNESS_ROLES_<ROLE>_CLAUDE` and `HARNESS_ROLES_<ROLE>_CODEX` for the 5 roles with a default, such as `HARNESS_ROLES_LEAD_CLAUDE`. A comma list, such as `sonnet,opus`, sets several models | The models for each role and host |

The roles, and the agent names that the guard maps to each role:

| Role | Agent names |
|---|---|
| `lead` | none: the lead is the main session. It defaults to `opus` on `claude` and `gpt-6-astra` on `codex`. The model is guidance only. The harness cannot switch the session model and no hook receives it, so the guard checks no agent call for it. A light-track run can use `sonnet` as the lead. |
| `explorer` | `explore`, `explorer`, `claude-code-guide`, `verifier`: a verifier with no model would run on the lead model |
| `worker` | `worker`, `general-purpose`, `claude`: the last two have every tool and can edit files |
| `reviewer` | `reviewer`, `second-reviewer`, `code-reviewer`, `pr-reviewer` |
| `advisor` | `advisor`: the check of a plan. It never counts as a reviewer for `review-models`. `harness plan check` prints `advisor: on, model <m>`. It names the first model on a full track and the second on a path trigger. A role with one model uses it for both. |

The guard compares the agent name in lower case, after the last `:`, so `90p:explorer` is `explorer`. An agent with another name is not checked.

The hosts:

| Host | Hook config |
|---|---|
| `claude` | `.claude/settings.json` |
| `codex` | `.codex/hooks.json` |

An unknown role or host is an error. A new role or a new agent name needs a code change in `internal/shared/infrastructure/config/config.go` and `internal/guard/domain/rules_agent.go`. YAML alone does not add one.

The file merges with the defaults for each role and host. A host that you set replaces the default model of that host only. The other defaults stay.

```yaml
roles:
  reviewer:
    claude: [sonnet, opus]
  worker:
    claude: sonnet
```

## `guard`

The guard rules that run on each hook. Each rule is on by default.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `disabled` | list of rule ids | empty | `HARNESS_GUARD_DISABLED` | The rules to turn off. An unknown id is an error that names it |

The rule ids:

| Rule | Event | Denies |
|---|---|---|
| `pr-no-merge` | `tool.before` | `gh pr merge`. `harness pr policy` prints `merge: user` while this rule is on |
| `pr-draft-only` | `tool.before` | `gh pr create` without `--draft`, and `gh pr ready`. `harness pr policy` prints `draft: required` while this rule is on |
| `pr-body-shape` | `tool.before` | A PR body with a problem. The sections `## Why`, `## What`, `## How`, `## Evidence` and `## Merge danger` are missing or out of order. The guard also denies a body with no bottom line above `## Why`. It denies an Evidence with no `Before:` line or no `After:` line, or a line with no proof below it. It denies an `After:` line above the `Before:` line. It denies a Merge danger with no `Door:` line or no `Blast radius:` line, a value outside its list, or no reason. It denies a body that keeps a placeholder of the skeleton. It denies a body with a line above 40 words. It denies a body with a diagram that has no marks. It denies a body with an em dash, on any line. The deny lists each problem. For a long line, it names the line number, the word count and the first 6 words. `harness pr check --body-file <path>` prints the same problems. |
| `git-no-force` | `tool.before` | A forced `git push` |
| `git-no-push-default` | `tool.before` | A `git push` to the default branch (see `git`) |
| `git-no-verify-skip` | `tool.before` | `--no-verify` on `git commit` or `git push` |
| `secrets-read` | `tool.before` | A read of a secret file, or a shell command that prints the process environment. The secret files are `.env`, `.env.*` other than `.env.example`, `id_rsa*`, `*.pem`, `*.key` and `/proc/<pid>/environ`. The commands are bare `env`, `printenv`, bare `export` or `export -p`, and bare `set`. The commands also include bare `declare` or `typeset`, either with `-p`, or with other flags and no name (not `-f` or `-F`). The rule also denies an `env` whose `-S` string, split into words, leaves no command. The rule also reads subshells, substitutions and `bash -c` strings. |
| `test-no-focus-skip` | `tool.before` | A focused or skipped test in a test edit |
| `shell-write-source` | `tool.before` | A shell write to a source file. The rule expands variables and follows the `cd` of the same command only on a flat line. A flat line is simple commands joined by `;`, `&&`, `\|\|`, `\|`, `&` or a newline. The rule expands a variable or follows a `cd` only when the assignment or `cd` surely runs. A word such as `set`, `export` or `eval`, a subshell, a substitution or an unquoted heredoc makes a line not flat. A write command with an option outside its allow list, such as `cp -t`, makes a line not flat. A write command with a word that the rule cannot resolve also makes a line not flat. For BSD `sed -i ''`, the empty word after `-i` is the backup suffix, not a target. The first operand is the sed script, and the files follow it. On a flat line, the deny names the file. For a target it cannot resolve, it names that target and asks for a literal absolute path. On a line that is not flat, the deny names the source target. When the target holds `$NAME`, `${...}`, `$(...)`, a backtick part or a leading `~`, the deny names each part. It then asks for a literal absolute path. Otherwise it asks for the Edit or Write tool. A write whose path starts with `.harness/` or `.kodify/` passes. Use the edit tools of the host |
| `writing-style` | `tool.before` | An em dash, an emoji or a Co-Authored-By line in commit or PR text |
| `agent-model-routing` | `tool.before` | An agent call on a model that is not the model of its role (see `roles`) |
| `commits-checked` | `tool.before` | `git push` or `gh pr create` before `harness verify --commits` passes for `HEAD` |
| `approval-no-forge` | `tool.before` | A question tool call with answers in its input, or a shell call of `harness hook` |
| `stop-holds-turn` | `turn.stopping` | The end of an agent turn in a phase that holds the turn (see `run.max_stop_blocks`). In `shipping`, the turn may end once `harness run wait --pr` records the pull request |
| `review-models` | `run.transition.before` | `harness run ship` when a model of the final panel has no counted review, or fewer than 2 models have a record (1 on a light panel). The rule picks the host list with the most recorded models. A hook records the review when a `reviewer` agent stops in the reviewing phase, in the common list and in the final panel. A review counts only when its transcript shows a read of a changed file, and the deny names the model that did not. A verifier call and a re-review write no record. A Codex repository turns it off, because Codex sends no model in an agent call |
| `review-findings` | `run.transition.before` | `harness run ship` while `findings.md` of the run has a real open blocker or major finding. With `run.strict_minor` on, a real open minor finding also denies. A missing file allows |

```yaml
guard:
  disabled: [pr-no-merge, pr-draft-only]
```

## `deploy`

The default deploy plugins. Each one proposes gates to `harness gates init` when its files exist.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `disabled` | list of plugin names | empty | `HARNESS_DEPLOY_DISABLED` | The default plugins to turn off. An unknown name is an error |

The plugins:

| Plugin | Needs | Proposes |
|---|---|---|
| `helm` | a `Chart.yaml` | the gate `helm-render`. It also renders the chart for the `observability` lint pack |
| `promtool` | a `*.rules.test.yaml` file | the gate `promtool-test` |

Only one plugin renders a chart. When your own plugin listens to `chart.render`, add `helm` to `deploy.disabled`, else the config is an error.

```yaml
deploy:
  disabled: [helm]
```

## `git`

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `default_branch` | string | empty | `HARNESS_GIT_DEFAULT_BRANCH` | The default branch. Empty reads `origin/HEAD`, else the first of `main` and `master` that exists, else `main`. The rule `git-no-push-default` and the review trace use it |

```yaml
git:
  default_branch: main
```

## `setup`

Steps that `harness verify` runs before the gates, such as a dependency install. A step has no evidence and no `needs`. A failed step stops `harness verify` before any gate.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `id` | string | required | none | Lower case letters, digits and `-`, starting with a letter. Unique in the list |
| `run` | string | required | none | The shell command. Its timeout is always `10m` |
| `paths` | list of globs | empty | none | The step runs again only when a matching file changed since its last pass. Empty means each file of the tree |
| `creates` | path | empty | none | A file or folder inside the project that the step makes. When it is missing, the step runs again |

```yaml
setup:
  - id: node-install
    run: pnpm install --frozen-lockfile
    paths: [package.json, pnpm-lock.yaml]
    creates: node_modules
```

## `gates`

The checks of `harness verify`. A gate passes on exit code 0. `harness verify --commits <base>` must pass before a push.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `id` | string | required | none | Lower case letters, digits and `-`, starting with a letter. Unique. The prefix `criterion-` is kept for the plan criteria |
| `run` | string | required | none | The shell command |
| `paths` | list of globs | empty | none | The gate result stays `CACHED` while no matching file changes. Empty means each file of the tree |
| `timeout` | duration | `10m` | none | The longest run. `0` gives `10m`. A negative value is an error |
| `baseline` | path | empty | none | A findings report. The gate fails only on a finding that is not in this file. The `lint` gate uses `.harness-lint/baseline.json` |
| `needs` | list of gate ids | empty | none | The gate starts only when each need is `PASS` or `CACHED`. Else it is `SKIPPED`. A need on itself or on an unknown id is an error |

`harness verify --criterion <n>` runs the setup steps and only the proof of plan criterion `n`, with no configured gate. It saves the evidence like `harness verify`. A number with no criterion is an error. The flag cannot go with `--commits`.

`harness gates init` detects the gates of the toolchain and writes this key. It refuses to change a `gates` key that holds a gate.

```yaml
gates:
  - id: go-vet
    run: go vet ./...
  - id: go-test
    run: go test ./...
    paths: ["**/*.go", go.mod, go.sum]
    timeout: 15m
    needs: [go-vet]
```

## `verify`

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `parallel` | int, 1 or more | `1` | `HARNESS_VERIFY_PARALLEL` | The most gates that `harness verify` runs at the same time. `1` runs them in file order |

```yaml
verify:
  parallel: 3
```

## `lint`

The architecture rules of `harness lint`. The [DESIGN.md lint packs section](../DESIGN.md#lint-packs) lists each rule.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `packs` | list | empty | none | The built-in pack names below, and the paths of custom pack folders (an entry with a `/`). A pack that needs a missing pack is an error |
| `disabled` | list | empty | `HARNESS_LINT_DISABLED` | The default lint plugins (`harness`, `biome-grit`, `oxlint`) and the rule ids, such as `ddd/domain-purity`, to turn off. An unknown entry is an error |
| `ddd.root` | string | `contexts` | `HARNESS_LINT_DDD_ROOT` | The business root folder in `src/` |
| `ddd.domain_allow` | list | empty | none | The packages that a domain file can import. A name also allows its sub paths |
| `ddd.src_files` | list | `[main.ts]` | `HARNESS_LINT_DDD_SRC_FILES` | The files that can be directly in `src/` |
| `shadcn.ui_allow` | list | empty | none | More modules that a `components/ui/` file can import |
| `observability.chart` | path | `deploy/chart` | `HARNESS_LINT_OBSERVABILITY_CHART` | The Helm chart folder |
| `observability.rules` | glob | `deploy/**/*.rules.yaml` | `HARNESS_LINT_OBSERVABILITY_RULES` | The alert rule files |
| `observability.dashboards` | glob | `deploy/**/dashboards/*.json` | `HARNESS_LINT_OBSERVABILITY_DASHBOARDS` | The dashboard files |

The built-in packs:

| Pack | Checks | Needs |
|---|---|---|
| `ddd` | The layout, the suffixes and the imports of a domain driven design project | none |
| `ddd-es` | The event sourcing rules: event versions and apply methods | `ddd` |
| `nextjs` | A Next.js App Router project | none |
| `shadcn` | The imports and the class names of shadcn components | none |
| `nextjs-ddd-shadcn` | The joins of Next.js, domain driven design and shadcn | `ddd`, `nextjs`, `shadcn` |
| `observability` | OpenTelemetry in the app, and the probes, image tags, alerts and dashboards of the chart | none |

```yaml
lint:
  packs: [ddd]
  disabled: [ddd/domain-purity]
  ddd:
    root: modules
    domain_allow: [uuid, date-fns]
```

## `live`

The go-live keys. They take effect only with `run.go_live: true`. [go-live](go-live.md) gives the flow.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `environment` | string | `production` | `HARNESS_LIVE_ENVIRONMENT` | The GitHub deployment environment that the run waits for |
| `rollback` | string | empty | `HARNESS_LIVE_ROLLBACK` | The command that the findings show on a regression. The harness never runs it |

```yaml
live:
  environment: production
  rollback: helm rollback api
```

## `panel`

The web panel, which shows the plans and the runs of the repository. `panel.enabled` is the master switch. It gates the autostart and the host open. The explicit `harness panel` commands work with any value.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `enabled` | bool | `false` | `HARNESS_PANEL_ENABLED` | `true` lets the panel autostart and open with a run. `false` turns both off and keeps the other keys |
| `port` | int, 0 to 65535 | `0` | `HARNESS_PANEL_PORT` | The port of the panel. `0` lets the system pick a free port. Any other value is a fixed port |
| `autostart` | bool | `false` | `HARNESS_PANEL_AUTOSTART` | `true` starts the panel after `harness run start` succeeds. A failed start prints a warning and the run goes on |
| `open` | string, `browser` or `host` | `browser` | `HARNESS_PANEL_OPEN` | `browser` opens the plan page in the browser. `host` prints the run URL on stderr and opens no browser |

A port outside 0 to 65535 is an error. So is an `open` value other than `browser` or `host`.

```yaml
panel:
  enabled: true
  port: 8765
  autostart: true
  open: host
```

## How to configure

### A new repository

1. Run `harness init --host claude` (or `--host codex`) at the repository root. It chooses `lint.packs`, writes `gates`, writes the lint files, installs the hooks and adds `.harness/runs/` to `.gitignore`.
2. Read its output. A line `next: run the lint-setup skill` names a language that needs the `lint-setup` skill.
3. Commit `.harness.yaml`, `.harness-lint/`, `.gitignore` and the host config.

The [setup page](setup.md#step-by-step) gives the step by step setup without `harness init`.

### A gate with paths and needs

1. Add the gate to `gates` with an `id` and a `run` command.
2. Set `paths` to the files that the gate reads, so a change in other files does not run the gate again.
3. Set `needs` to the gates that must pass first.
4. Run `harness verify` and read the summary line. The `cached` count shows the gates that did not run again.

```yaml
gates:
  - id: unit
    run: make test
    paths: ["src/**", Makefile]
  - id: e2e
    run: make e2e
    needs: [unit]
```

### The model of each role

1. Set the models under the role and the host: one model, or a list.
2. Start the agent with a model from that list. Else the rule `agent-model-routing` denies the call and names the model.

```yaml
roles:
  explorer:
    claude: haiku
  reviewer:
    claude: [sonnet, haiku, opus]
```

### Turn off one guard rule

1. Find the rule id in the deny reason. The bus writes it as `guard.<id>`.
2. Add the id to `guard.disabled`.
3. Commit the change with the reason in the commit message. A comment in the file also helps the next reader.

```yaml
guard:
  disabled: [pr-no-merge]
```

### A Codex repository

1. Run `harness init --host codex`. It writes the hooks into `.codex/hooks.json`.
2. The default roles name Codex models. The guard lets a Codex agent call with no model pass, because Codex sends none. It still denies a wrong model.
3. Add `review-models` to `guard.disabled`. Codex sends no model in an agent call. Without this setting, `harness run ship` is always denied.
4. Model names go stale. Check `~/.codex/models_cache.json` for the local Codex model list, and override any name in `.harness.yaml`.
5. Commit `.harness.yaml` and `.codex/hooks.json`.

### A lint pack setting

1. Add the pack to `lint.packs`.
2. Set the keys of the pack, such as `lint.ddd.root`.
3. Run `harness lint init`, then `harness lint` to see the findings.
4. Run `harness lint baseline` to keep the findings of today. The `lint` gate then fails only on a new finding.

```yaml
lint:
  packs: [ddd, observability]
  ddd:
    root: contexts
  observability:
    chart: charts/api
```

### Check a value

1. Run `harness config` to see `state`, `run`, `plugins`, `roles`, `guard` and `git`.
2. For another key, run the command that reads it, such as `harness verify` or `harness lint`. An invalid value stops it with an error that names the key.
