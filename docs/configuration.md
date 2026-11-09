# Configuration

This page explains each key of `.harness.yaml` and how to configure it. Each key gives its type, its default, its env variable and its effect.

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
├── the push target of the guard → `git`
└── the state database → `state`
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

## `state`

The run state database. It holds the runs, the plan approvals, the gate results and the commit checks.

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
| `max_stop_blocks` | int, 1 or more | `3` | `HARNESS_RUN_MAX_STOP_BLOCKS` | How many times in a row the guard refuses the end of an agent turn in a phase that holds the turn |

A plan with only `docs` commits and no open question skips the approval, also with `approval: true`.

```yaml
run:
  approval: true
  max_review_rounds: 5
```

## `plugins`

Your own commands on the event bus. The harness writes the event envelope as JSON on stdin and reads one verdict from stdout. The [README plugins section](../README.md#plugins) gives the verdict format and four examples.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `name` | string | required | none | The plugin name. Unique in the list. The bus calls the plugins of one event in name order |
| `command` | list of strings | required | none | The program and its arguments. The harness runs it with no shell |
| `events` | list of strings | required | none | One event name or more, from the table below. An unknown name is an error |
| `timeout` | duration | `5s` | none | The longest run of one call. `0` gives `5s`. A negative value is an error |
| `transition` | list of strings | none | none | Optional. Only for the events `run.transition.before` and `run.transition.after`; any other event with it is an error. The plugin runs only for these transitions: `start`, `submit-plan`, `approve-plan`, `amend`, `request-rework`, `submit-for-review`, `approve-review`, `complete`, `deploy`, `close`, `regress`, `block`, `resume`, `abandon`. A value in one key is an OR. The three keys together are an AND |
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

The keys `transition`, `from` and `to` pick the run transitions that a plugin sees. The transition names are `start`, `submit-plan`, `approve-plan`, `amend`, `request-rework`, `submit-for-review`, `approve-review`, `complete`, `deploy`, `close`, `regress`, `block`, `resume` and `abandon`. The phase names are `planning`, `awaiting_approval`, `working`, `reviewing`, `shipping`, `releasing`, `observing`, `blocked`, `done` and `abandoned`.

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

The rule `review-models` needs 2 different reviewer models on `HEAD`. So a host that lists a `reviewer` role needs 2 or more models there, or `review-models` in `guard.disabled`. With one model, `harness run ship` is always denied.

| Key | Type | Default | Env | Effect |
|---|---|---|---|---|
| `roles` | map of role to a map of host to a list of models | `explorer.claude: [haiku]`, `worker.claude: [sonnet, haiku]`, `reviewer.claude: [sonnet, haiku, opus]`, `advisor.claude: [fable]` | `HARNESS_ROLES_<ROLE>_CLAUDE` for the 4 roles with a default. A comma list, such as `sonnet,opus`, sets several models | The models for each role and host |

The roles, and the agent names that the guard maps to each role:

| Role | Agent names |
|---|---|
| `lead` | none: the lead is the main session, so the guard checks no agent call for it |
| `explorer` | `explore`, `explorer`, `general-purpose`, `claude-code-guide` |
| `worker` | `worker` |
| `reviewer` | `reviewer`, `second-reviewer`, `code-reviewer`, `pr-reviewer` |
| `advisor` | `advisor`: the check of a plan. It never counts as a reviewer for `review-models` |

The guard compares the agent name in lower case, after the last `:`, so `harness:explorer` is `explorer`. An agent with another name is not checked.

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
| `pr-no-merge` | `tool.before` | `gh pr merge` |
| `pr-draft-only` | `tool.before` | `gh pr create` without `--draft` |
| `pr-body-shape` | `tool.before` | A PR body without `## Why`, `## What` and `## How` in this order, a line above 40 words, or a diagram with no marks |
| `git-no-force` | `tool.before` | A forced `git push` |
| `git-no-push-default` | `tool.before` | A `git push` to the default branch (see `git`) |
| `git-no-verify-skip` | `tool.before` | `--no-verify` on `git commit` or `git push` |
| `secrets-read` | `tool.before` | A read of a secret file |
| `test-no-focus-skip` | `tool.before` | A focused or skipped test in a test edit |
| `shell-write-source` | `tool.before` | A shell write to a source file. Use the edit tools of the host |
| `writing-style` | `tool.before` | An em dash, an emoji or a Co-Authored-By line in commit or PR text |
| `agent-model-routing` | `tool.before` | An agent call on a model that is not the model of its role (see `roles`) |
| `commits-checked` | `tool.before` | `git push` or `gh pr create` before `harness verify --commits` passes for `HEAD` |
| `approval-no-forge` | `tool.before` | A question tool call with answers in its input, or a shell call of `harness hook` |
| `stop-holds-turn` | `turn.stopping` | The end of an agent turn in a phase that holds the turn (see `run.max_stop_blocks`) |
| `review-models` | `run.transition.before` | `harness run ship` when fewer than 2 different models reviewed `HEAD`. A hook records the model of each `reviewer` agent call in the reviewing phase. A Codex repository turns it off, because Codex sends no model in an agent call |

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
| `default_branch` | string | empty | `HARNESS_GIT_DEFAULT_BRANCH` | The default branch. Empty reads `origin/HEAD`, else `main`. The rule `git-no-push-default` and the review trace use it |

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

## How to configure

### A new repository

1. Run `harness init --host claude` (or `--host codex`) at the repository root. It chooses `lint.packs`, writes `gates`, writes the lint files and installs the hooks.
2. Read its output. A line `next: run the lint-setup skill` names a language that needs the `lint-setup` skill.
3. Add `.harness/` to `.gitignore`.
4. Commit `.harness.yaml`, `.harness-lint/` and the host config.

The [README](../README.md#set-up-a-repository) gives the step by step setup without `harness init`.

### A gate with paths and needs

1. Add the gate to `gates` with an `id` and a `run` command.
2. Set `paths` to the files that the gate reads, so a change in other files gives `CACHED`.
3. Set `needs` to the gates that must pass first.
4. Run `harness verify` and read the result line of each gate.

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
2. The default roles name only the `claude` host, so the guard checks no model on Codex. To check one, set the Codex model under the role, as `roles.reviewer.codex`.
3. Add `review-models` to `guard.disabled`. Codex sends no model in an agent call, so else `harness run ship` is always denied.
4. Commit `.harness.yaml` and `.codex/hooks.json`.

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
