# Roadmap

Eleven pull requests follow the skeleton (PR 0). Each PR branches from the one before it. Each later run copies its PR block into its own plan.

| PR | Branch | Scope | Fixes |
|---|---|---|---|
| 0 | `feat/harness-skeleton` | this run | K1, K9, K12 |
| 1 | `feat/state-dbal-sqlite` | DBAL, SQLite dialect, run store | K14 |
| 2 | `feat/config-viper` | viper config, role table for each host | K5 |
| 3 | `feat/run-commands` | `harness run` commands on the store, spans for each transition | none |
| 4 | `feat/plan-module` | plan parse and checks in any phase, persisted track, diagram check in Go, waves, trace | K7, K10, K13 |
| 4b | `feat/marked-diagram` | `harness diagram` with `+ * -` marks, diagram mark rule | none |
| 5 | `feat/host-adapters` | Claude Code and Codex hook codecs, declared capabilities | K8 |
| 6 | `feat/guard-plugins` | built-in guard rules as in-process plugins | K2 |
| 7 | `feat/gates-evidence` | YAML and criterion gates, baseline ratchet, evidence for each gate input hash, `commits-checked` rule, trace veto on review | K4, K6, K16 |
| 8 | `feat/lint-packs` | `lint` module and the six packs | K3 |
| 9 | `feat/live-loop` | `live` module, `signals.check`, regression | K11 |
| 10 | `feat/plan-view` | self-contained plan page, trace with verification state, change colors, UI sketches | K15 |

## PR 1: `feat/state-dbal-sqlite`, based on `feat/harness-skeleton`

- Scope: the DBAL with the embedded SQLite dialect and the run store with optimistic versions.
- Fixes: K14.

| # | Criterion | Test |
|---|---|---|
| 1.1 | `Open` with an unknown DSN scheme returns `ErrUnknownEngine`. | `go test ./internal/shared/infrastructure/dbal -run TestOpenUnknownEngine -count=1` |
| 1.2 | A SQLite database opens with WAL, a 5000 ms busy timeout and foreign keys on. | `go test ./internal/shared/infrastructure/dbal -run TestOpenSQLitePragmas -count=1` |
| 1.3 | `Migrate` applies only the files above the stored module version, in number order, and stores the new version. | `go test ./internal/shared/infrastructure/dbal -run TestMigrateAppliesOnlyGap -count=1` |
| 1.4 | A migration file with a bad name returns `ErrBadMigrationName`, and the schema does not change. | `go test ./internal/shared/infrastructure/dbal -run TestMigrateBadFileName -count=1` |
| 1.5 | `InTx` rolls back when the function returns an error. | `go test ./internal/shared/infrastructure/dbal -run TestInTxRollback -count=1` |
| 1.6 | `Apply` records one `Transitioned` event with the old phase, the event, the new phase, the reason and the command time. | `go test ./internal/run/domain -run TestApplyRecordsTransition -count=1` |
| 1.7 | A saved run reads back equal, with version 1 and no events. | `go test ./internal/run/infrastructure/persistence -run TestRunRepositoryContract/sqlite/SaveAndGet -count=1` |
| 1.8 | `Active` returns the open run of the key, and `ErrRunNotFound` for a key with no open run. | `go test ./internal/run/infrastructure/persistence -run TestRunRepositoryContract/sqlite/ActiveByKey -count=1` |
| 1.9 | A save with an old version returns `ErrStaleRun`, and the stored run does not change. | `go test ./internal/run/infrastructure/persistence -run TestRunRepositoryContract/sqlite/StaleVersion -count=1` |
| 1.10 | A second open run for the same key returns `ErrActiveRunExists`. A done run does not count. | `go test ./internal/run/infrastructure/persistence -run TestRunRepositoryContract/sqlite/OneActivePerKey -count=1` |
| 1.11 | `Save` writes the run row and its transitions in one transaction, so a failed save writes neither. | `go test ./internal/run/infrastructure/persistence -run TestRunRepositoryContract/sqlite/TransitionsAtomic -count=1` |
| 1.12 | The module is formatted, builds, vets and passes all tests with no error. | `make check` |

Tasks:

1. **DBAL and SQLite dialect.** Owned: `internal/shared/infrastructure/dbal/`, `./go.mod`, `./go.sum`. Test first: `dbal_test.go` with `TestMigrateAppliesOnlyGap`, which fails because the package does not exist. DoD: criteria 1.1 to 1.5 and 1.12. Depends on: none.
2. **Run transitions and the repository port.** Owned: `internal/run/domain/`. Test first: `run_test.go` with `TestApplyRecordsTransition`, which fails because `Run.Events` does not exist. DoD: criterion 1.6. Depends on: none.
3. **SQL run repository.** Owned: `internal/run/infrastructure/persistence/`. Test first: `repository_test.go` with `TestRunRepositoryContract`, which fails because `SQLRunRepository` does not exist. DoD: criteria 1.7 to 1.11. Depends on: tasks 1 and 2.

Commits:

1. `feat: add the DBAL with the embedded SQLite dialect` (task 1)
2. `feat: record run transitions and declare the run repository` (task 2)
3. `feat: store runs in the DBAL with optimistic versions` (task 3)

Code tree:

```tree
harness/
├── go.mod                                  changed
├── go.sum                                  changed
└── internal/
    ├── shared/
    │   └── infrastructure/
    │       └── dbal/
    │           ├── dbal.go                 new
    │           ├── dialect.go              new
    │           ├── errors.go               new
    │           ├── migrate.go              new
    │           ├── sqlite.go               new
    │           └── dbal_test.go            new
    └── run/
        ├── domain/
        │   ├── command.go                  changed
        │   ├── errors.go                   changed
        │   ├── repository.go               new
        │   ├── run.go                      changed
        │   ├── run_test.go                 changed
        │   └── transitioned.go             new
        └── infrastructure/
            └── persistence/
                ├── migrations/
                │   └── sqlite/
                │       └── 0001_create_runs.sql new
                ├── repository.go           new
                └── repository_test.go      new
```

## PR 2: `feat/config-viper`, based on `feat/state-dbal-sqlite`

- Scope: the viper config loader, the role table for each host, the `harness config` command and the split of the exit codes.
- Fixes: K5.

| # | Criterion | Test |
|---|---|---|
| 2.1 | With no file, no env and no flag, `Load` returns the defaults of the spec. | `go test ./internal/shared/infrastructure/config -run TestLoadDefaults -count=1` |
| 2.2 | For `state.dsn`, a flag wins over the env, the env wins over the file and the file wins over the default. | `go test ./internal/shared/infrastructure/config -run TestLoadPrecedence -count=1` |
| 2.3 | An unknown key in the file is an error that names the key. | `go test ./internal/shared/infrastructure/config -run TestLoadUnknownKey -count=1` |
| 2.4 | Invalid YAML is an error that names the file. | `go test ./internal/shared/infrastructure/config -run TestLoadInvalidYAML -count=1` |
| 2.5 | A plugin with no timeout gets 5 seconds, and `750ms` parses as 750 milliseconds. | `go test ./internal/shared/infrastructure/config -run TestLoadPluginTimeout -count=1` |
| 2.6 | An unknown role or host in the role table is an error. | `go test ./internal/shared/infrastructure/config -run TestLoadUnknownRole -count=1` |
| 2.7 | A `max_*` value below 1 is an error. | `go test ./internal/shared/infrastructure/config -run TestLoadInvalidRounds -count=1` |
| 2.8 | `TopLevel` returns the repo root from a subfolder, and an error outside a repo. | `go test ./internal/shared/infrastructure/git -run TestTopLevel -count=1` |
| 2.9 | `harness config` prints the effective config as YAML, with each plugin timeout as a duration string, and exits 0. | `go test ./cmd/harness -run TestConfigCommand -count=1` |
| 2.10 | `harness config --config missing.yaml` exits 1 with an error that names the file. | `go test ./cmd/harness -run TestConfigFlagMissingFile -count=1` |
| 2.11 | A usage error exits 2, and a runtime error exits 1. | `go test ./cmd/harness -run TestExitCodes -count=1` |
| 2.12 | The envelope JSON stays the same after the host type moves to the shared domain. | `go test ./internal/shared/domain -run TestEnvelopeJSON -count=1` |
| 2.13 | The module is formatted, builds, vets and passes all tests with no error. | `make check` |

Tasks:

1. **Shared host and config loader.** Owned: `internal/shared/domain/`, `internal/shared/infrastructure/config/`, `internal/shared/domain/envelope.go`, `./go.mod`, `./go.sum`. Test first: `config_test.go` with `TestLoadDefaults`, which fails because the package does not exist. DoD: criteria 2.1 to 2.7 and 2.12. Depends on: none.
2. **Git top level.** Owned: `internal/shared/infrastructure/git/`. Test first: `git_test.go` with `TestTopLevel`, which fails because the package does not exist. DoD: criterion 2.8. Depends on: none.
3. **Config command and exit codes.** Owned: `cmd/harness/`, `./DESIGN.md`. Test first: `main_test.go` with `TestExitCodes`, which fails because every error exits 2. DoD: criteria 2.9 to 2.11 and 2.13. Depends on: tasks 1 and 2.

Commits:

1. `feat: load the harness config with viper` (task 1)
2. `feat: find the git top level of the repo` (task 2)
3. `feat: add the config command and split the exit codes` (task 3)

Code tree:

```tree
harness/
├── DESIGN.md                               changed
├── go.mod                                  changed
├── go.sum                                  changed
├── cmd/
│   └── harness/
│       ├── config.go                       new
│       ├── main.go                         changed
│       └── main_test.go                    changed
└── internal/
    ├── shared/
    │   ├── domain/
    │   │   └── host.go                     new
    │   └── infrastructure/
    │       ├── config/
    │       │   ├── config.go               new
    │       │   └── config_test.go          new
    │       └── git/
    │           ├── git.go                  new
    │           └── git_test.go             new
    └── plugin/
        └── domain/
            └── envelope.go                 changed
```

## PR 3: `feat/run-commands`, based on `feat/config-viper`

- Scope: the `harness run` commands on the store, with the transition hooks on the plugin bus and a span for each transition.
- Fixes: none.

| # | Criterion | Test |
|---|---|---|
| 3.1 | `RepoKey` normalizes the SSH, HTTPS and `ssh://` forms of the origin URL to one host and path. | `go test ./internal/shared/infrastructure/git -run TestRepoKey -count=1` |
| 3.2 | Without an `origin` remote, the repo key is the absolute git common dir. | `go test ./internal/shared/infrastructure/git -run TestRepoKeyNoOrigin -count=1` |
| 3.3 | The run id is the clock time as `20060102-150405`, a dash and a slug of at most 40 characters. | `go test ./internal/run/application/command/start -run TestNewRunID -count=1` |
| 3.4 | `start` saves a new run in `planning` with the prompt. | `go test ./internal/run/application/command/start -run TestStartNewRun -count=1` |
| 3.5 | `start` with an open run returns `ErrActiveRunExists`, and saves nothing. | `go test ./internal/run/application/command/start -run TestStartWithActiveRun -count=1` |
| 3.6 | The `transition` handler calls `Before`, `Apply`, `Save` and `After` in this order. `After` gets the old and the new run. | `go test ./internal/run/application/command/transition -run TestTransitionOrder -count=1` |
| 3.7 | A veto in `Before` returns `ErrTransitionVetoed` with the reason. The handler saves nothing and calls no `After`. | `go test ./internal/run/application/command/transition -run TestTransitionVetoed -count=1` |
| 3.8 | An illegal event returns `ErrIllegalTransition`, and the handler saves nothing. | `go test ./internal/run/application/command/transition -run TestTransitionIllegal -count=1` |
| 3.9 | `ErrStaleRun` from `Save` goes to the caller, and the handler calls no `After`. | `go test ./internal/run/application/command/transition -run TestTransitionStale -count=1` |
| 3.10 | Each transition records one `run.transition` span with the run id, the event and both phases. A veto sets the status to error. | `go test ./internal/run/application/command/transition -run TestTransitionSpan -count=1` |
| 3.11 | The status query returns a `Status` value, and `ErrRunNotFound` with no open run. | `go test ./internal/run/application/query/status -run TestStatusQuery -count=1` |
| 3.12 | `Before` dispatches `run.transition.before` with the run reference and the payload, and maps a deny to `ErrTransitionVetoed`. | `go test ./internal/run/infrastructure/pluginhooks -run TestHooksBeforeDeny -count=1` |
| 3.13 | `After` dispatches `run.transition.after` with the old and the new phase. | `go test ./internal/run/infrastructure/pluginhooks -run TestHooksAfterEnvelope -count=1` |
| 3.14 | `FromConfig` returns `ErrUnknownEvent` for a configured plugin with an unknown event name. | `go test ./internal/shared/infrastructure/execplugin -run TestFromConfigUnknownEvent -count=1` |
| 3.15 | A saved run reads back with its prompt after the migration `0002`. | `go test ./internal/run/infrastructure/persistence -run TestRunRepositoryContract/sqlite/SaveAndGet -count=1` |
| 3.16 | The status output has the run line, the reason line only when set, and the `next:` hint of the phase. | `go test ./internal/run/ui/cli -run TestStatusLines -count=1` |
| 3.17 | In a temp git repo, `start`, `plan`, `approve`, `review`, `ship` and `done` end the run, and `status` prints `active: no`. | `go test ./cmd/harness -run TestRunLifecycle -count=1` |
| 3.18 | The default state file is `.harness/state.db` in the git common dir, also when the command runs in a linked worktree. | `go test ./cmd/harness -run TestStateInGitCommonDir -count=1` |
| 3.19 | A `ui` package imports no `infrastructure` package. | `go test . -run TestLayerImports -count=1` |
| 3.20 | The module is formatted, builds, vets and passes all tests with no error. | `make check` |

Tasks:

1. **Repo key.** Owned: `internal/shared/infrastructure/git/`. Test first: `git_test.go` with `TestRepoKey`, which fails because `RepoKey` does not exist. DoD: criteria 3.1 and 3.2. Depends on: none.
2. **Run prompt and hooks port.** Owned: `internal/run/domain/`, `internal/run/infrastructure/persistence/`. Test first: `repository_test.go` with the `Prompt` case, which fails because the column does not exist. DoD: criterion 3.15. Depends on: none.
3. **Run use cases.** Owned: `internal/run/application/`. Test first: `transition/handler_test.go` with `TestTransitionOrder`, which fails because the package does not exist. DoD: criteria 3.3 to 3.11. Depends on: task 2.
4. **Plugin hooks and bus from config.** Owned: `internal/run/infrastructure/pluginhooks/`, `internal/shared/infrastructure/execplugin/config.go`, `internal/shared/infrastructure/execplugin/config_test.go`. Test first: `pluginhooks/hooks_test.go` with `TestHooksBeforeDeny`, which fails because the package does not exist. DoD: criteria 3.12 to 3.14. Depends on: task 2.
5. **Run CLI and wiring.** Owned: `internal/run/ui/`, `internal/run/module.go`, `cmd/harness/`, `./architecture_test.go`. Test first: `cmd/harness/main_test.go` with `TestRunLifecycle`, which fails because the `run` command does not exist. DoD: criteria 3.16 to 3.20. Depends on: tasks 1, 3 and 4.

Commits:

1. `feat: derive the run key from the git remote` (task 1)
2. `feat: store the run prompt and declare the transition hooks` (task 2)
3. `feat: add the run use cases with transition spans` (task 3)
4. `feat: dispatch run transitions to plugins` (task 4)
5. `feat: add the run commands to the CLI` (task 5)

Code tree:

```tree
harness/
├── architecture_test.go                    changed
├── cmd/
│   └── harness/
│       ├── main.go                         changed
│       └── main_test.go                    changed
└── internal/
    ├── shared/
    │   └── infrastructure/
    │       └── git/
    │           ├── git.go                  changed
    │           └── git_test.go             changed
    ├── plugin/
    │   ├── module.go                       new
    │   └── module_test.go                  new
    └── run/
        ├── module.go                       new
        ├── domain/
        │   ├── hooks.go                    new
        │   └── run.go                      changed
        ├── application/
        │   ├── command/
        │   │   ├── start/
        │   │   │   ├── handler.go          new
        │   │   │   └── handler_test.go     new
        │   │   └── transition/
        │   │       ├── handler.go          new
        │   │       └── handler_test.go     new
        │   └── query/
        │       └── status/
        │           ├── handler.go          new
        │           └── handler_test.go     new
        ├── infrastructure/
        │   ├── persistence/
        │   │   ├── migrations/
        │   │   │   └── sqlite/
        │   │   │       └── 0002_add_run_prompt.sql new
        │   │   ├── repository.go           changed
        │   │   └── repository_test.go      changed
        │   └── pluginhooks/
        │       ├── hooks.go                new
        │       └── hooks_test.go           new
        └── ui/
            └── cli/
                ├── commands.go             new
                └── commands_test.go        new
```

## PR 4: `feat/plan-module`, based on `feat/run-commands`

- Scope: parse and check `plan.md` in any phase, persist the track, compute the waves and the trace, and send `plan.submitted`.
- Fixes: K7, K10, K13.

| # | Criterion | Test |
|---|---|---|
| 4.1 | A valid fixture plan parses into criteria, tasks, trees, commits and PRs, each with its line number. | `go test ./internal/plan/domain -run TestParsePlan -count=1` |
| 4.2 | Each check rule reports one problem in the form `plan.md:12: rule: text` for its broken fixture. The valid fixture has none. | `go test ./internal/plan/domain -run TestCheckRules -count=1` |
| 4.3 | The sentence rule counts a backticked span as one word, ends a sentence only at `.`, `!` or `?`, and skips code fences. | `go test ./internal/plan/domain -run TestSentenceLength -count=1` |
| 4.4 | `Waves` groups the tasks by dependency level. A cycle returns `ErrDependencyCycle` with the task numbers. | `go test ./internal/plan/domain -run TestWaves -count=1` |
| 4.5 | `Classify` gives `full` for each trigger of the spec and `light` else. A declared `light` does not lower a `full` plan. | `go test ./internal/plan/domain -run TestClassifyTrack -count=1` |
| 4.6 | `submit-plan` and `amend` set the run track to the stricter of the run track and the plan track. | `go test ./internal/run/domain -run TestRunTrackNeverLowers -count=1` |
| 4.7 | A code tree with a command, query, endpoint or consumer file needs `## Signals` with all six columns and a Go duration window. | `go test ./internal/plan/domain -run TestSignalsRequired -count=1` |
| 4.8 | With a live round above 0, the plan needs `## Regression impact` that names an archived plan file that exists. | `go test ./internal/plan/domain -run TestRegressionImpact -count=1` |
| 4.9 | The Go parser accepts the diagrams of `DESIGN.md` and refuses a second label colon, a bad edge and the `root` id. | `go test ./internal/plan/infrastructure/mermaid -run TestParseDiagrams -count=1` |
| 4.10 | `harness plan check` works in each phase and with no run. It exits 0 on a clean plan and 1 with problems on stdout. | `go test ./internal/plan/ui/cli -run TestPlanCheckCommand -count=1` |
| 4.11 | `harness run plan` sends `plan.submitted`. A deny or a plugin failure stops the submit and prints the reasons. | `go test ./internal/plan/application/command/submitplan -run TestSubmitPlanCollect -count=1` |
| 4.12 | The check records one `plan.check` span with the problem count and the track. | `go test ./internal/plan/application/query/checkplan -run TestCheckPlanSpan -count=1` |
| 4.13 | `Trace` links each criterion to its tasks, spec sections, proof test, commits, PR and signal row. `Unlinked` holds the items with no link. | `go test ./internal/plan/domain -run TestTraceLinks -count=1` |
| 4.14 | The rules `spec-criteria` and `trace-unlinked` report one problem each for their broken fixtures. | `go test ./internal/plan/domain -run TestTraceRules -count=1` |

Tasks:

1. **Plan parser.** Owned: `internal/plan/domain/document.go`, `internal/plan/domain/parse.go`, `internal/plan/domain/problem.go`, `internal/plan/domain/parse_test.go`, `internal/plan/domain/testdata/`. Test first: `parse_test.go` with `TestParsePlan`, which fails because `Parse` does not exist. DoD: criterion 4.1. Depends on: none.
2. **Plan checks and trace.** Owned: `internal/plan/domain/rule.go`, `internal/plan/domain/check.go`, `internal/plan/domain/sentence.go`, `internal/plan/domain/trace.go` and their tests. Test first: `check_test.go` with `TestCheckRules`, which fails because `Check` does not exist. DoD: criteria 4.2, 4.3, 4.7, 4.8, 4.13 and 4.14. Depends on: 1.
3. **Track and waves.** Owned: `internal/plan/domain/track.go`, `internal/plan/domain/waves.go` and their tests, `internal/run/domain/run.go`, `internal/run/domain/run_test.go`. Test first: `waves_test.go` with `TestWaves`, which fails because `Waves` does not exist. DoD: criteria 4.4, 4.5 and 4.6. Depends on: 1.
4. **Mermaid parser.** Owned: `internal/plan/infrastructure/mermaid/`. Test first: `parse_test.go` with `TestParseDiagrams`, which fails because the package does not exist. DoD: criterion 4.9. Depends on: none.
5. **Check query, submit command and CLI.** Owned: `internal/plan/application/`, `internal/plan/ui/cli/`, `internal/plan/module.go`, `cmd/harness/main.go`. Test first: `submitplan/handler_test.go` with `TestSubmitPlanCollect`, which fails because the handler does not exist. DoD: criteria 4.10 to 4.12. Depends on: 2, 3 and 4.

Commits:

1. `feat: parse the plan document` (task 1)
2. `feat: check the plan rules and trace the links` (task 2)
3. `feat: keep the stricter track and compute the waves` (task 3)
4. `feat: check mermaid diagrams without Node` (task 4)
5. `feat: add harness plan check and the plan submitted event` (task 5)

Code tree:

```tree
harness/
├── cmd/
│   └── harness/
│       └── main.go                         changed
└── internal/
    ├── plan/
    │   ├── module.go                       new
    │   ├── domain/
    │   │   ├── check.go                    new
    │   │   ├── document.go                 new
    │   │   ├── parse.go                    new
    │   │   ├── problem.go                  new
    │   │   ├── rule.go                     new
    │   │   ├── sentence.go                 new
    │   │   ├── trace.go                    new
    │   │   ├── track.go                    new
    │   │   └── waves.go                    new
    │   ├── application/
    │   │   ├── command/
    │   │   │   └── submitplan/
    │   │   │       └── handler.go          new
    │   │   └── query/
    │   │       └── checkplan/
    │   │           └── handler.go          new
    │   ├── infrastructure/
    │   │   └── mermaid/
    │   │       └── parse.go                new
    │   └── ui/
    │       └── cli/
    │           └── check.go                new
    └── run/
        └── domain/
            └── run.go                      changed
```

## PR 5: `feat/host-adapters`, based on `feat/marked-diagram`

- Scope: Claude Code and Codex hook codecs, the `harness hook` and `harness host` commands, declared capabilities and fallbacks.
- Fixes: K8.

| # | Criterion | Test |
|---|---|---|
| 5.1 | The Claude Code codec decodes the fixture of each of the 7 hooks to its bus event and payload. | `go test ./internal/host/infrastructure/hookcodec -run TestClaudeDecode -count=1` |
| 5.2 | The Claude Code codec encodes allow, deny and context for each event to the output JSON of the spec. An allow prints nothing. | `go test ./internal/host/infrastructure/hookcodec -run TestClaudeEncode -count=1` |
| 5.3 | The Codex codec decodes the input of each of the 7 hooks, and gives `edit` as the kind of `apply_patch`. | `go test ./internal/host/infrastructure/hookcodec -run TestCodexDecode -count=1` |
| 5.4 | The Codex codec encodes the verdicts like Claude Code, and an allow on `stop` and on `subagent-stop` prints `{}`. | `go test ./internal/host/infrastructure/hookcodec -run TestCodexEncode -count=1` |
| 5.5 | Each codec declares 7 events. `ToolKindOf` maps each tool name of both hosts to its kind. | `go test ./internal/host/domain -run TestCapabilities -count=1` |
| 5.6 | For a fake codec with fewer events, the `session-start` payload names each missing event. The context gives one line for each. | `go test ./internal/host/application/command/handlehook -run TestMissingEventFallback -count=1` |
| 5.7 | A harness error on a veto event gives a deny with `harness error`. On another event it gives one stderr line and the allow output. | `go test ./internal/host/application/command/handlehook -run TestHookFailsClosed -count=1` |
| 5.8 | Input above 16 MiB is a harness error. | `go test ./internal/host/application/command/handlehook -run TestHookInputLimit -count=1` |
| 5.9 | Each hook records one `hook.handle` span and one `harness.hook.duration` point with the host, hook and verdict attributes. | `go test ./internal/host/application/command/handlehook -run TestHandleHookTelemetry -count=1` |
| 5.10 | `harness hook --host claude pre-tool-use` with a deny plugin prints the deny JSON and exits 0. | `go test ./internal/host/ui/cli -run TestHookCommand -count=1` |
| 5.11 | `harness host install --host claude` merges the hooks into `.claude/settings.json`, keeps the other keys, and a second run changes nothing. | `go test ./internal/host/infrastructure/install -run TestInstallClaude -count=1` |
| 5.12 | `harness host install --host codex` merges the hooks into `.codex/hooks.json` with the same rules. | `go test ./internal/host/infrastructure/install -run TestInstallCodex -count=1` |

Tasks:

1. **Host domain and payloads.** Owned: `internal/host/domain/`, `internal/shared/domain/payload.go`, `ROADMAP.md`, `DESIGN.md`. Test first: `capability_test.go` with `TestCapabilities`, which fails because the package does not exist. DoD: criterion 5.5. Depends on: none.
2. **Claude Code codec.** Owned: `internal/host/infrastructure/hookcodec/codec.go` and `claude_test.go`. Test first: `claude_test.go` with `TestClaudeDecode`, which fails because the package does not exist. DoD: criteria 5.1 and 5.2. Depends on: 1.
3. **Codex codec.** Owned: `internal/host/infrastructure/hookcodec/codex_test.go`. Test first: `codex_test.go` with `TestCodexDecode`, which fails because the package does not exist. DoD: criteria 5.3 and 5.4. Depends on: 1.
4. **Hook handler and command.** Owned: `internal/host/application/`, `internal/host/ui/cli/hook.go`, `internal/host/ui/cli/hook_test.go`, `internal/host/module.go`, `internal/shared/application/eventbus/bus.go`, `cmd/harness/`. Test first: `handler_test.go` with `TestHookFailsClosed`, which fails because the handler does not exist. DoD: criteria 5.6 to 5.10, and the hook command test. Depends on: 2, 3 and 5.
5. **Install.** Owned: `internal/host/infrastructure/install/`, `internal/host/ui/cli/host.go`, `internal/host/ui/cli/host_test.go`. Test first: `install_test.go` with `TestInstallClaude`, which fails because the package does not exist. DoD: criteria 5.11 and 5.12. Depends on: 1.

Commits:

1. `feat: add the host domain and the event payloads` (task 1)
2. `feat: add the Claude Code hook codec` (task 2)
3. `feat: add the Codex hook codec` (task 3)
4. `feat: handle host hooks through the event bus` (task 4)
5. `feat: install the harness hooks for each host` (task 5)

Code tree:

```tree
harness/
├── ROADMAP.md                              changed
├── DESIGN.md                               changed
├── cmd/
│   └── harness/
│       ├── host.go                         new
│       └── main.go                         changed
└── internal/
    ├── host/
    │   ├── module.go                       new
    │   ├── domain/
    │   │   ├── capability.go               new
    │   │   ├── codec.go                    new
    │   │   └── hook.go                     new
    │   ├── application/
    │   │   └── command/
    │   │       └── handlehook/
    │   │           └── handler.go          new
    │   ├── infrastructure/
    │   │   ├── hookcodec/
    │   │   │   └── codec.go                new
    │   │   └── install/
    │   │       ├── claude.json             new
    │   │       ├── codex.json              new
    │   │       └── install.go              new
    │   └── ui/
    │       └── cli/
    │           ├── hook.go                 new
    │           └── host.go                 new
    └── shared/
        ├── application/
        │   └── eventbus/
        │       └── bus.go                  changed
        └── domain/
            └── payload.go                  new
```

## PR 6: `feat/guard-plugins`, based on `feat/host-adapters`

- Scope: the built-in guard rules as in-process plugins, with a config switch for each rule. The rules are `pr-no-merge`, `pr-draft-only`, `pr-body-shape`, `git-no-force`, `git-no-push-default`, `git-no-verify-skip`, `secrets-read`, `test-no-focus-skip`, `shell-write-source`, `writing-style`, `agent-model-routing` and `stop-holds-turn`.
- Fixes: K2.

| # | Criterion | Test |
|---|---|---|
| 6.1 | `Commands` splits a shell line at `&&`, `\|\|`, `;`, pipes and newlines, and keeps quoted text whole. | `go test ./internal/guard/domain -run TestShellCommands -count=1` |
| 6.2 | Each rule denies each listed match and allows each listed near miss of its table. | `go test ./internal/guard/domain -run TestRuleMatches -count=1` |
| 6.3 | Each bus deny starts with `guard.`, the rule id and a colon once, and names the next step. | `go test ./internal/guard/application/guardplugin -run TestDenyReasonShape -count=1` |
| 6.4 | Each rule is one plugin named `guard.` plus its id, subscribed only to its event. | `go test ./internal/guard/application/guardplugin -run TestGuardPluginNames -count=1` |
| 6.5 | A rule in `guard.disabled` makes no plugin. An unknown id in the list is a config error. | `go test ./internal/guard/application/guardplugin -run TestGuardDisabled -count=1` |
| 6.6 | `agent-model-routing` reads the role table of the envelope host from the config. | `go test ./internal/guard/application/guardplugin -run TestAgentModelRouting -count=1` |
| 6.7 | `stop-holds-turn` blocks as `DecideStop` says for the active run and saves the new stop block count. | `go test ./internal/guard/application/guardplugin -run TestStopHoldsTurn -count=1` |
| 6.8 | Each deny adds 1 to `harness.guard.denies` with the `harness.rule` attribute. | `go test ./internal/guard/application/guardplugin -run TestGuardDenyCounter -count=1` |
| 6.9 | `DefaultBranch` reads `origin/HEAD` in a test repo, and gives `main` when it is not set. | `go test ./internal/shared/infrastructure/git -run TestDefaultBranch -count=1` |
| 6.10 | `harness hook --host claude pre-tool-use` with a `git push --force` input prints the `git-no-force` deny. | `go test ./cmd/harness -run TestHookGuardEndToEnd -count=1` |

Tasks:

1. **Shell and tool call parse.** Owned: `internal/guard/domain/shell.go`, `internal/guard/domain/toolcall.go` and their tests. Test first: `shell_test.go` with `TestShellCommands`, which fails because `Commands` does not exist. DoD: criterion 6.1. Depends on: none.
2. **Rules.** Owned: `internal/guard/domain/` except the files of task 1. Test first: `rule_test.go` with `TestRuleMatches`, which fails because the rules do not exist. DoD: criteria 6.2 and 6.3. Depends on: 1.
3. **Default branch.** Owned: `internal/shared/infrastructure/git/`. Test first: `git_test.go` with `TestDefaultBranch`, which fails because `DefaultBranch` does not exist. DoD: criterion 6.9. Depends on: none.
4. **Guard plugins and wiring.** Owned: `internal/guard/application/`, `internal/guard/module.go`, `cmd/harness/main.go`, `cmd/harness/main_test.go`. Test first: `plugin_test.go` with `TestGuardPluginNames`, which fails because the package does not exist. DoD: criteria 6.4 to 6.8 and 6.10. Depends on: 2 and 3.

Commits:

1. `feat: parse shell lines and tool calls for the guard` (task 1)
2. `feat: add the built-in guard rules` (task 2)
3. `feat: read the default branch from git` (task 3)
4. `feat: run the guard rules as bus plugins` (task 4)

Code tree:

```tree
harness/
├── cmd/
│   └── harness/
│       └── main.go                         changed
└── internal/
    └── guard/
        ├── module.go                       new
        ├── domain/
        │   ├── rule.go                     new
        │   ├── rules_agent.go              new
        │   ├── rules_files.go              new
        │   ├── rules_git.go                new
        │   ├── rules_pr.go                 new
        │   ├── rules_stop.go               new
        │   ├── rules_text.go               new
        │   ├── shell.go                    new
        │   └── toolcall.go                 new
        ├── application/
        │   └── guardplugin/
        │       └── plugin.go               new
└── shared/
    └── infrastructure/
        └── git/
            └── git.go                      changed
```

## PR 7: `feat/gates-evidence`, based on `main`

- Scope: generic verification gates from YAML and one gate for each criterion, the evidence store for each input hash, and `harness verify`. It also adds the `commits-checked` guard rule and the trace verification before the review.
- Fixes: K4, K6, K16.

| # | Criterion | Test |
|---|---|---|
| 7.1 | An empty id, an id that breaks the pattern, a duplicate id or an empty run is an error. The error names the gate. | `go test ./internal/gate/domain -run TestNewGateValidation -count=1` |
| 7.2 | Only the error findings whose rule, file and message are not in the baseline are new. A moved line is not new. | `go test ./internal/gate/domain -run TestBaselineNewFindings -count=1` |
| 7.3 | A `test:` commit that passes, a later fix that fails, or a failed other commit breaks the commit rules. | `go test ./internal/gate/domain -run TestJudgeCommits -count=1` |
| 7.4 | The input hash changes with a matching file and stays the same with a file outside the paths. The real index does not change. | `go test ./internal/gate/infrastructure/gittree -run TestInputHash -count=1` |
| 7.5 | The shell runner kills a gate at its timeout, returns `fail`, and keeps the last 4 KiB of the output. | `go test ./internal/gate/infrastructure/runner -run TestShellRunnerTimeout -count=1` |
| 7.6 | The evidence and commit check adapters pass the PR 1 contract suite on SQLite. | `go test ./internal/gate/infrastructure/persistence -run TestEvidenceRepositoryContract -count=1` |
| 7.7 | `Verify` runs each gate once, saves its evidence, and fails when one gate fails. | `go test ./internal/gate/application/command/verify -run TestVerifyRunsGates -count=1` |
| 7.8 | A gate with a `pass` record for its input hash is `cached` and does not run. | `go test ./internal/gate/application/command/verify -run TestVerifyReusesEvidence -count=1` |
| 7.9 | A gate with a `fail` record for its input hash runs again. | `go test ./internal/gate/application/command/verify -run TestVerifyRerunsFailure -count=1` |
| 7.10 | A `gate.before` deny gives `vetoed` and the gate does not run. A `gate.after` plugin gets the result. | `go test ./internal/gate/application/command/verify -run TestVerifyGateEvents -count=1` |
| 7.11 | Each gate records one `gate.run` span with its attributes. Only a gate that runs records a `harness.gate.duration` point. | `go test ./internal/gate/application/command/verify -run TestVerifyTelemetry -count=1` |
| 7.12 | `VerifyCommits` checks each commit in a temporary worktree, saves the commit check for `HEAD`, and removes the worktrees. | `go test ./internal/gate/application/command/verifycommits -run TestVerifyCommits -count=1` |
| 7.13 | A Go repo gets three Go gates. A pnpm repo with scripts and Biome gets the pnpm and Biome gates. | `go test ./internal/gate/infrastructure/toolchain -run TestDetectToolchain -count=1` |
| 7.14 | `InitGates` writes the gates when the key is absent, and returns `ErrGatesExist` when it exists. | `go test ./internal/gate/application/command/initgates -run TestInitGatesRefusesOverwrite -count=1` |
| 7.15 | `commits-checked` denies `git push` and `gh pr create` without a passed commit check for `HEAD`, and allows them with one. | `go test ./internal/guard/application/guardplugin -run TestCommitsCheckedRule -count=1` |
| 7.16 | `harness verify` prints one status line for each gate, exits 0 when all pass, and exits 1 when one fails. | `go test ./internal/gate/ui/cli -run TestVerifyCommand -count=1` |
| 7.17 | `Verify` adds one `criterion-N` gate for each gate of the `GateSource`, and saves the evidence of each one. | `go test ./internal/gate/application/command/verify -run TestVerifyCriterionGates -count=1` |
| 7.18 | `VerifyCommits` runs only the YAML gates, and no `criterion-N` gate. | `go test ./internal/gate/application/command/verifycommits -run TestVerifyCommitsSkipsCriteria -count=1` |
| 7.19 | `VerifyTrace` reports one problem for each of the six trace rules on its broken fixture, and none on the clean fixture. | `go test ./internal/plan/domain -run TestVerifyTrace -count=1` |
| 7.20 | The `gitdiff` adapter reads the name status, the commits with their files and the test sources of a temporary git repo. | `go test ./internal/plan/infrastructure/gitdiff -run TestReadBranch -count=1` |
| 7.21 | The review plugin denies `submit-for-review` with the trace problems, allows a clean branch, and ignores the other events. | `go test ./internal/plan/infrastructure/reviewplugin -run TestReviewPlugin -count=1` |
| 7.22 | In a temporary git repo, `harness run review` refuses a file outside the trees, and moves the run to `reviewing` after the fix. | `go test ./cmd/harness -run TestReviewVerifiesTrace -count=1` |
| 7.23 | `EvidenceStatus` gives the result of each gate for its current input hash, and no result when the record is missing. | `go test ./internal/gate/application/query/evidencestatus -run TestEvidenceStatus -count=1` |
| 7.24 | The config decodes the `gates` key into gates, and a bad gate is an error that names it. | `go test ./internal/gate/infrastructure/config -run TestDecodeGates -count=1` |
| 7.25 | The module is formatted, builds, vets and passes all tests with no error. | `make check` |
| 7.26 | `harness gates init` writes the detected gates, and exits 1 when the `gates` key exists. | `go test ./internal/gate/ui/cli -run TestGatesInitCommand -count=1` |

Tasks:

1. **Gate domain.** Owned: `internal/gate/domain/`. Test first: `gate_test.go` with `TestNewGateValidation`, which fails because `NewGate` does not exist. DoD: criteria 7.1 to 7.3. Depends on: none.
2. **Git tree and shell runner.** Owned: `internal/gate/infrastructure/gittree/`, `internal/gate/infrastructure/runner/`, `internal/shared/infrastructure/git/git.go`, `internal/shared/infrastructure/git/git_test.go`. Test first: `tree_test.go` with `TestInputHash`, which fails because the package does not exist. DoD: criteria 7.4 and 7.5. Depends on: task 1.
3. **Evidence store.** Owned: `internal/gate/infrastructure/persistence/`. Test first: `evidence_repository_test.go` with `TestEvidenceRepositoryContract`, which fails because the adapter does not exist. DoD: criterion 7.6. Depends on: task 1.
4. **Verify use cases.** Owned: `internal/gate/application/command/verify/`, `internal/gate/application/command/verifycommits/`, `internal/gate/application/query/evidencestatus/`. Test first: `handler_test.go` with `TestVerifyRunsGates`, which fails because the handler does not exist. DoD: criteria 7.7 to 7.12, 7.17, 7.18 and 7.23. Depends on: tasks 2 and 3.
5. **Gates init.** Owned: `internal/gate/infrastructure/toolchain/`, `internal/gate/infrastructure/config/`, `internal/gate/application/command/initgates/`, `internal/shared/infrastructure/config/config.go`, `internal/shared/infrastructure/config/config_test.go`. Test first: `detect_test.go` with `TestDetectToolchain`, which fails because `Detect` does not exist. DoD: criteria 7.13, 7.14 and 7.24. Depends on: task 1.
6. **CLI, module, guard rule and wiring.** Owned: `internal/gate/ui/cli/`, `internal/gate/module.go`, `internal/guard/`, `cmd/harness/gate.go`, `cmd/harness/guard.go`, `cmd/harness/main.go`, `cmd/harness/run.go`, `cmd/harness/main_test.go`, `./DESIGN.md`, `./ROADMAP.md`. Test first: `guardplugin/plugin_test.go` with `TestCommitsCheckedRule`, which fails because the rule does not exist. DoD: criteria 7.15, 7.16, 7.22, 7.25 and 7.26. Depends on: tasks 4, 5 and 7.
7. **Trace verification.** Owned: `internal/plan/domain/verify.go`, `internal/plan/domain/verify_test.go`, `internal/plan/domain/document.go`, `internal/plan/domain/parse.go`, `internal/plan/domain/parse_test.go`, `internal/plan/domain/testdata/`, `internal/plan/infrastructure/gitdiff/`, `internal/plan/infrastructure/reviewplugin/`, `internal/plan/module.go`, `internal/shared/domain/transition.go`, `internal/run/domain/hooks.go`, `internal/run/domain/event.go`, `internal/run/infrastructure/pluginhooks/`, `internal/run/ui/cli/commands.go`, `cmd/harness/plan.go`. Test first: `verify_test.go` with `TestVerifyTrace`, which fails because `VerifyTrace` does not exist. DoD: criteria 7.19 to 7.21. Depends on: task 2.

Commits:

1. `feat: add the gate domain with baselines and commit rules` (task 1)
2. `feat: add the input hash and the shell gate runner` (task 2)
3. `feat: store the gate evidence and commit checks` (task 3)
4. `feat: verify the gates, the criteria and each commit with evidence reuse` (task 4)
5. `feat: detect the toolchain and write the default gates` (task 5)
6. `feat: verify the plan trace before the review` (task 7)
7. `feat: add harness verify, the commits-checked rule and the review veto` (task 6)

Code tree:

```tree
harness/
├── DESIGN.md                                       changed
├── ROADMAP.md                                      changed
├── cmd/
│   └── harness/
│       ├── gate.go                                 new
│       ├── guard.go                                changed
│       ├── main.go                                 changed
│       ├── main_test.go                            changed
│       ├── plan.go                                 changed
│       └── run.go                                  changed
└── internal/
    ├── guard/
    │   ├── module.go                               changed
    │   ├── domain/
    │   │   ├── rule.go                             changed
    │   │   ├── rule_test.go                        changed
    │   │   └── rules_commits.go                    new
    │   └── application/
    │       └── guardplugin/
    │           ├── plugin.go                       changed
    │           └── plugin_test.go                  changed
    ├── plan/
    │   ├── module.go                               changed
    │   ├── domain/
    │   │   ├── document.go                         changed
    │   │   ├── parse.go                            changed
    │   │   ├── parse_test.go                       changed
    │   │   ├── verify.go                           new
    │   │   ├── verify_test.go                      new
    │   │   └── testdata/
    │   │       ├── verify_broken.md                new
    │   │       └── verify_clean.md                 new
    │   └── infrastructure/
    │       ├── gitdiff/
    │       │   ├── branch.go                       new
    │       │   └── branch_test.go                  new
    │       └── reviewplugin/
    │           ├── plugin.go                       new
    │           └── plugin_test.go                  new
    ├── run/
    │   ├── domain/
    │   │   ├── event.go                            changed
    │   │   └── hooks.go                            changed
    │   ├── infrastructure/
    │   │   └── pluginhooks/
    │   │       ├── hooks.go                        changed
    │   │       └── hooks_test.go                   changed
    │   └── ui/
    │       └── cli/
    │           └── commands.go                     changed
    ├── shared/
    │   ├── domain/
    │   │   └── transition.go                       new
    │   └── infrastructure/
    │       ├── config/
    │       │   ├── config.go                       changed
    │       │   └── config_test.go                  changed
    │       └── git/
    │           ├── git.go                          changed
    │           └── git_test.go                     changed
    └── gate/
        ├── module.go                               new
        ├── domain/
        │   ├── commits.go                          new
        │   ├── commits_test.go                     new
        │   ├── errors.go                           new
        │   ├── evidence.go                         new
        │   ├── finding.go                          new
        │   ├── finding_test.go                     new
        │   ├── gate.go                             new
        │   ├── gate_test.go                        new
        │   ├── repository.go                       new
        │   └── source.go                           new
        ├── application/
        │   ├── command/
        │   │   ├── initgates/
        │   │   │   ├── handler.go                  new
        │   │   │   └── handler_test.go             new
        │   │   ├── verify/
        │   │   │   ├── handler.go                  new
        │   │   │   └── handler_test.go             new
        │   │   └── verifycommits/
        │   │       ├── handler.go                  new
        │   │       └── handler_test.go             new
        │   └── query/
        │       └── evidencestatus/
        │           ├── handler.go                  new
        │           └── handler_test.go             new
        ├── infrastructure/
        │   ├── config/
        │   │   ├── gates.go                        new
        │   │   └── gates_test.go                   new
        │   ├── gittree/
        │   │   ├── tree.go                         new
        │   │   └── tree_test.go                    new
        │   ├── persistence/
        │   │   ├── commit_check_repository.go      new
        │   │   ├── evidence_repository.go          new
        │   │   ├── evidence_repository_test.go     new
        │   │   └── migrations/
        │   │       └── sqlite/
        │   │           └── 0001_create_evidence.sql new
        │   ├── runner/
        │   │   ├── shell.go                        new
        │   │   └── shell_test.go                   new
        │   └── toolchain/
        │       ├── detect.go                       new
        │       └── detect_test.go                  new
        └── ui/
            └── cli/
                ├── gates.go                        new
                ├── gates_test.go                   new
                ├── verify.go                       new
                └── verify_test.go                  new
```

## PR 8: `feat/lint-packs`, based on `main`

- Scope: the `lint` module with the six packs on Biome, oxlint and the harness layout engine, with the baseline ratchet.
- Fixes: K3.

| # | Criterion | Test |
|---|---|---|
| 8.1 | Each of the six packs loads from the embedded files. Each rule has a unique id, a known engine, a known severity and a scope. | `go test ./internal/lint/infrastructure/packs -run TestPackCatalogue -count=1` |
| 8.2 | `ddd-es` without `ddd`, or `nextjs-ddd-shadcn` without its three packs, returns `ErrMissingPack`. | `go test ./internal/lint/domain -run TestPackRequires -count=1` |
| 8.3 | The import parser resolves relative paths, `tsconfig.json` aliases and `index.ts`, and counts type-only imports. | `go test ./internal/lint/infrastructure/harnessengine -run TestImportGraph -count=1` |
| 8.4 | Each `harness` engine rule fixture gives exactly one finding with its rule id. Each `clean` fixture gives none. | `go test ./internal/lint/infrastructure/harnessengine -run TestHarnessEngineFixtures -count=1` |
| 8.5 | Each rendered chart rule fixture of the `observability` pack gives exactly one finding with its rule id. | `go test ./internal/lint/infrastructure/harnessengine -run TestObservabilityChartFixtures -count=1` |
| 8.6 | A golden Biome JSON report maps to findings with the file, the line and the rule id. | `go test ./internal/lint/infrastructure/biome -run TestBiomeReportMapping -count=1` |
| 8.7 | Each GritQL and Biome built-in rule fixture gives exactly one finding with its rule id. | `go test ./internal/lint/infrastructure/biome -run TestBiomeEngineFixtures -count=1` |
| 8.8 | Each oxlint rule fixture gives exactly one finding with its rule id, and each oxlint rule has a reason. | `go test ./internal/lint/infrastructure/oxlint -run TestOxlintEngineFixtures -count=1` |
| 8.9 | `harness lint --json` prints the version 1 report and exits 0. Without the flag, an `error` finding exits 1 and a `warn` finding does not. | `go test ./internal/lint/ui/cli -run TestLintCommandReport -count=1` |
| 8.10 | The lint run records one `lint.run` span, and a `harness.lint.findings` count for each rule and severity. The bus `plugin.call` span times each engine. | `go test ./internal/lint/application/command/lint -run TestLintTelemetry -count=1` |
| 8.11 | `InitLint` writes `.harness-lint/` with `VERSION`. `Lint` with an old `VERSION` returns `ErrStaleLintFiles`. | `go test ./internal/lint/application/command/initlint -run TestLintInitVersion -count=1` |
| 8.12 | The first baseline holds all findings. A later baseline only removes the fixed keys and never adds one. | `go test ./internal/lint/application/command/baseline -run TestBaselineRatchet -count=1` |
| 8.13 | `harness gates init` adds the `lint` gate with packs set, and the `helm-render` and `promtool-test` gates with `observability`. | `go test ./internal/gate/infrastructure/toolchain -run TestDetectLintGates -count=1` |
| 8.14 | Each rule id in the Lint packs tables of `DESIGN.md` is in the catalogue, and each catalogue rule is in the tables. | `go test . -run TestDesignLintRulesMatchCatalogue -count=1` |

Tasks:

1. **Lint domain.** Owned: `internal/lint/domain/`. Test first: `pack_test.go` with `TestPackRequires`, which fails because `CheckRequires` does not exist. DoD: criterion 8.2. Depends on: none.
2. **Pack catalogue.** Owned: `internal/lint/infrastructure/packs/`, `internal/lint/infrastructure/config/`. Test first: `catalogue_test.go` with `TestPackCatalogue`, which fails because the embedded files do not exist. DoD: criterion 8.1. Depends on: task 1.
3. **Harness engine.** Owned: `internal/lint/infrastructure/harnessengine/`. Test first: `imports_test.go` with `TestImportGraph`, which fails because the parser does not exist. DoD: criteria 8.3 to 8.5. Depends on: task 2.
4. **Biome engine.** Owned: `internal/lint/infrastructure/biome/`. Test first: `report_test.go` with `TestBiomeReportMapping`, which fails because the mapping does not exist. DoD: criteria 8.6 and 8.7. Depends on: task 2.
5. **Oxlint engine.** Owned: `internal/lint/infrastructure/oxlint/`. Test first: `runner_test.go` with `TestOxlintEngineFixtures`, which fails because the runner does not exist. DoD: criterion 8.8. Depends on: task 2.
6. **Lint use cases, CLI and module.** Owned: `internal/lint/application/`, `internal/lint/ui/cli/`, `internal/lint/module.go`, `cmd/harness/main.go`, `internal/gate/infrastructure/toolchain/detect.go`, `internal/gate/infrastructure/toolchain/detect_test.go`. Test first: `lint_test.go` with `TestLintCommandReport`, which fails because the command does not exist. DoD: criteria 8.9 to 8.13. Depends on: tasks 3, 4 and 5.
7. **Docs and CI.** Owned: `./DESIGN.md`, `./docs_test.go`, `.github/workflows/lint-engines.yml`. Test first: `docs_test.go` with `TestDesignLintRulesMatchCatalogue`, which fails because the tables are not in `DESIGN.md`. DoD: criterion 8.14. Depends on: task 2.

Commits:

1. `feat: add the lint domain with packs, rules and findings` (task 1)
2. `feat: embed the six lint packs` (task 2)
3. `feat: add the harness layout and import graph engine` (task 3)
4. `feat: run the GritQL and Biome rules of the packs` (task 4)
5. `feat: run the oxlint rules of the packs` (task 5)
6. `feat: add harness lint with init and the baseline ratchet` (task 6)
7. `docs: describe the lint packs and check them against the catalogue` (task 7)

Code tree:

```tree
harness/
├── DESIGN.md                                       changed
├── docs_test.go                                    changed
├── .github/
│   └── workflows/
│       └── lint-engines.yml                        new
├── cmd/
│   └── harness/
│       └── main.go                                 changed
└── internal/
    ├── gate/
    │   └── infrastructure/
    │       └── toolchain/
    │           ├── detect.go                       changed
    │           └── detect_test.go                  changed
    └── lint/
        ├── module.go                               new
        ├── domain/
        │   ├── errors.go                           new
        │   ├── eventsourcing.go                    new
        │   ├── eventsourcing_test.go               new
        │   ├── finding.go                          new
        │   ├── graph.go                            new
        │   ├── imports.go                          new
        │   ├── imports_test.go                     new
        │   ├── layout.go                           new
        │   ├── layout_test.go                      new
        │   ├── pack.go                             new
        │   ├── pack_test.go                        new
        │   ├── rule.go                             new
        │   └── tree.go                             new
        ├── application/
        │   └── command/
        │       ├── baseline/
        │       │   ├── command.go                  new
        │       │   ├── handler.go                  new
        │       │   └── handler_test.go             new
        │       ├── initlint/
        │       │   ├── command.go                  new
        │       │   ├── handler.go                  new
        │       │   └── handler_test.go             new
        │       └── lint/
        │           ├── command.go                  new
        │           ├── handler.go                  new
        │           └── handler_test.go             new
        ├── infrastructure/
        │   ├── biome/
        │   │   ├── config.go                       new
        │   │   ├── report.go                       new
        │   │   ├── report_test.go                  new
        │   │   ├── runner.go                       new
        │   │   ├── runner_test.go                  new
        │   │   └── testdata/                       new
        │   ├── config/
        │   │   ├── lint.go                         new
        │   │   └── lint_test.go                    new
        │   ├── harnessengine/
        │   │   ├── engine.go                       new
        │   │   ├── engine_test.go                  new
        │   │   ├── files.go                        new
        │   │   ├── helm.go                         new
        │   │   ├── helm_test.go                    new
        │   │   ├── imports.go                      new
        │   │   ├── imports_test.go                 new
        │   │   ├── tsconfig.go                     new
        │   │   └── testdata/                       new
        │   ├── oxlint/
        │   │   ├── report.go                       new
        │   │   ├── runner.go                       new
        │   │   ├── runner_test.go                  new
        │   │   └── testdata/                       new
        │   └── packs/
        │       ├── catalogue.go                    new
        │       ├── catalogue_test.go               new
        │       ├── ddd/
        │       │   ├── README.md                   new
        │       │   ├── pack.yaml                   new
        │       │   └── rules/
        │       │       ├── aggregate-encapsulation.grit    new
        │       │       ├── command-handler-returns.grit    new
        │       │       ├── event-readonly.grit             new
        │       │       ├── publish-after-save.grit         new
        │       │       ├── query-handler-no-write.grit     new
        │       │       ├── value-object-equals.grit        new
        │       │       └── value-object-factory.grit       new
        │       ├── ddd-es/
        │       │   ├── README.md                   new
        │       │   ├── pack.yaml                   new
        │       │   └── rules/
        │       │       └── event-version.grit      new
        │       ├── nextjs/
        │       │   ├── README.md                   new
        │       │   └── pack.yaml                   new
        │       ├── nextjs-ddd-shadcn/
        │       │   ├── README.md                   new
        │       │   └── pack.yaml                   new
        │       ├── observability/
        │       │   ├── README.md                   new
        │       │   └── pack.yaml                   new
        │       └── shadcn/
        │           ├── README.md                   new
        │           ├── pack.yaml                   new
        │           └── rules/
        │               ├── cn-classnames.grit      new
        │               └── no-raw-colors.grit      new
        └── ui/
            └── cli/
                ├── lint.go                         new
                └── lint_test.go                    new
```

## PR 9: `feat/live-loop`, based on `feat/lint-packs`

- Scope: the `live` module that follows a release to production, checks its signals in the watch window, and regresses or closes the run.
- Fixes: K11.

| # | Criterion | Test |
|---|---|---|
| 9.1 | `ParseSignals` reads each row of `## Signals`. A bad operator, a value that is not a number or a bad window is an error that names the row. | `go test ./internal/live/domain -run TestParseSignals -count=1` |
| 9.2 | In `releasing`, an open PR or a pending deploy is `pending`, a failed deploy is `regression`, and a deploy is `deployed`. | `go test ./internal/live/domain -run TestDecideReleasing -count=1` |
| 9.3 | In `observing`, a breach is `regression` with every breach, an ended window with no breach is `healthy`, and an open window is `pending`. | `go test ./internal/live/domain -run TestDecideObserving -count=1` |
| 9.4 | Only failures for more than one window after the window end give `unavailable` and the `block` event. | `go test ./internal/live/domain -run TestDecideSignalsUnavailable -count=1` |
| 9.5 | The findings text lists each breach with its value, threshold, window and panel, then the rollback line or the not configured line. | `go test ./internal/live/domain -run TestFindingsText -count=1` |
| 9.6 | The GitHub adapter reads the merge SHA from `gh pr view` and the newest deployment status for the SHA and the environment. | `go test ./internal/live/infrastructure/github -run TestGitHubMergeAndDeploy -count=1` |
| 9.7 | The release adapter passes the PR 1 contract suite on SQLite. | `go test ./internal/live/infrastructure/persistence -run TestReleaseRepositoryContract -count=1` |
| 9.8 | `harness run done` without `--pr`, or with a value that is not a GitHub PR URL, is a usage error and exits 2. | `go test ./internal/run/ui/cli -run TestRunDonePR -count=1` |
| 9.9 | The `complete` event with a PR URL sets `PRURL` on the run and appends the URL to `PRs`. | `go test ./internal/run/domain -run TestCompleteRecordsPR -count=1` |
| 9.10 | `regress` clears the submitted tree, the reviewed tree, the checked commits and the PR URL, and keeps the PR list. | `go test ./internal/run/domain -run TestRegressClearsIteration -count=1` |
| 9.11 | `ListByPhase` returns only the runs of the repo in the given phases. The PR URL and the PR list survive a save and a load. | `go test ./internal/run/infrastructure/persistence -run TestListByPhase -count=1` |
| 9.12 | The check dispatches one `signals.check` for each signal row with the payload of the spec, and applies the decided event through the run store. | `go test ./internal/live/application/command/check -run TestLiveCheckDispatch -count=1` |
| 9.13 | A stale run version skips that run, reports it, and archives nothing. | `go test ./internal/live/application/command/check -run TestLiveCheckStaleRun -count=1` |
| 9.14 | Each checked run records one `live.check` span with its attributes, and adds 1 to `harness.live.checks` for its outcome. | `go test ./internal/live/application/command/check -run TestLiveCheckTelemetry -count=1` |
| 9.15 | A regression archives `plan.md`, `review.md` and the judgements with the old live round. An archive that exists returns `ErrArchiveExists`. | `go test ./internal/live/infrastructure/archive -run TestArchiveOnRegress -count=1` |
| 9.16 | The plan plugin denies an invalid `## Signals` table, and a plan that touches a handler or a route without `## Signals`. | `go test ./internal/live/infrastructure/planplugin -run TestSignalsPlanPlugin -count=1` |
| 9.17 | The session plugin adds one context line for each live run. With no live run, it calls no `gh` command. | `go test ./internal/live/infrastructure/sessionplugin -run TestLiveSessionContext -count=1` |
| 9.18 | `harness live check` prints one line for each live run and exits 0. An error that stops the pass exits 1. | `go test ./internal/live/ui/cli -run TestLiveCheckCommand -count=1` |

Tasks:

1. **Live domain.** Owned: `internal/live/domain/`. Test first: `threshold_test.go` with `TestParseSignals`, which fails because `ParseSignals` does not exist. DoD: criteria 9.1 to 9.5. Depends on: none.
2. **GitHub adapter.** Owned: `internal/live/infrastructure/github/`. Test first: `github_test.go` with `TestGitHubMergeAndDeploy`, which fails because the adapter does not exist. DoD: criterion 9.6. Depends on: task 1.
3. **Release store.** Owned: `internal/live/infrastructure/persistence/`. Test first: `release_repository_test.go` with `TestReleaseRepositoryContract`, which fails because the adapter does not exist. DoD: criterion 9.7. Depends on: task 1.
4. **Run PR record and live runs query.** Owned: `internal/run/domain/command.go`, `internal/run/domain/run.go`, `internal/run/domain/run_test.go`, `internal/run/domain/repository.go`, `internal/run/application/command/transition/`, `internal/run/infrastructure/persistence/`, `internal/run/ui/cli/`. Test first: `run_test.go` with `TestCompleteRecordsPR`, which fails because `Run` has no `PRURL` field. DoD: criteria 9.8 to 9.11. Depends on: none.
5. **Live check.** Owned: `internal/live/application/`, `internal/live/infrastructure/archive/`. Test first: `handler_test.go` with `TestLiveCheckDispatch`, which fails because the handler does not exist. DoD: criteria 9.12 to 9.15. Depends on: tasks 2, 3 and 4.
6. **Plan and session plugins.** Owned: `internal/live/infrastructure/planplugin/`, `internal/live/infrastructure/sessionplugin/`. Test first: `signals_test.go` with `TestSignalsPlanPlugin`, which fails because the plugin does not exist. DoD: criteria 9.16 and 9.17. Depends on: tasks 1 and 5.
7. **CLI, module and docs.** Owned: `internal/live/ui/cli/`, `internal/live/module.go`, `cmd/harness/main.go`, `docs/go-live.md`, `./DESIGN.md`. Test first: `live_test.go` with `TestLiveCheckCommand`, which fails because the command does not exist. DoD: criterion 9.18. Depends on: tasks 5 and 6.

Commits:

1. `feat: add the live domain with signals, decisions and findings` (task 1)
2. `feat: read the merge and the deploy from GitHub` (task 2)
3. `feat: store the releases of the live module` (task 3)
4. `feat: record the PR URL with harness run done and list the live runs` (task 4)
5. `feat: check the live runs and regress or close them` (task 5)
6. `feat: require plan signals and report live runs at session start` (task 6)
7. `feat: add harness live check and the go-live guide` (task 7)

Code tree:

```tree
harness/
├── DESIGN.md                                       changed
├── cmd/
│   └── harness/
│       └── main.go                                 changed
├── docs/
│   └── go-live.md                                  new
└── internal/
    ├── live/
    │   ├── module.go                               new
    │   ├── domain/
    │   │   ├── decision.go                         new
    │   │   ├── decision_test.go                    new
    │   │   ├── errors.go                           new
    │   │   ├── findings.go                         new
    │   │   ├── findings_test.go                    new
    │   │   ├── release.go                          new
    │   │   ├── repository.go                       new
    │   │   ├── threshold.go                        new
    │   │   └── threshold_test.go                   new
    │   ├── application/
    │   │   └── command/
    │   │       └── check/
    │   │           ├── command.go                  new
    │   │           ├── handler.go                  new
    │   │           └── handler_test.go             new
    │   ├── infrastructure/
    │   │   ├── archive/
    │   │   │   ├── archive.go                      new
    │   │   │   └── archive_test.go                 new
    │   │   ├── github/
    │   │   │   ├── deployments.go                  new
    │   │   │   ├── github_test.go                  new
    │   │   │   └── merge.go                        new
    │   │   ├── persistence/
    │   │   │   ├── release_repository.go           new
    │   │   │   ├── release_repository_test.go      new
    │   │   │   └── migrations/
    │   │   │       └── sqlite/
    │   │   │           └── 0001_releases.sql       new
    │   │   ├── planplugin/
    │   │   │   ├── signals.go                      new
    │   │   │   └── signals_test.go                 new
    │   │   └── sessionplugin/
    │   │       ├── live_context.go                 new
    │   │       └── live_context_test.go            new
    │   └── ui/
    │       └── cli/
    │           ├── live.go                         new
    │           └── live_test.go                    new
    └── run/
        ├── domain/
        │   ├── command.go                          changed
        │   ├── repository.go                       changed
        │   ├── run.go                              changed
        │   └── run_test.go                         changed
        ├── application/
        │   └── command/
        │       └── transition/
        │           ├── handler.go                  changed
        │           └── handler_test.go             changed
        ├── infrastructure/
        │   └── persistence/
        │       ├── repository.go                   changed
        │       ├── repository_test.go              changed
        │       └── migrations/
        │           └── sqlite/
        │               └── 0003_add_run_prs.sql    new
        └── ui/
            └── cli/
                ├── commands.go                     changed
                └── commands_test.go                changed
```

## PR 10: `feat/plan-view`, based on `feat/live-loop`

- Scope: `harness plan view`, a self-contained HTML page of the plan. It draws the trace with its verification state, change colors and UI sketches, and adds one check rule.
- Fixes: K15.

| # | Criterion | Test |
|---|---|---|
| 10.1 | `ParseMarks` in the shared domain reads the `class` and `cssClass` lines that `harness diagram` writes, with the line patterns of `CheckMarks`. It returns the mark of each node, and `CheckMarks` already refuses any other class name. | `go test ./internal/shared/domain -run TestParseMarks -count=1` |
| 10.2 | The rule `diagram-required` refuses each row of the spec table when its diagram or its `.excalidraw` file is missing. | `go test ./internal/plan/domain -run TestDiagramRequiredRule -count=1` |
| 10.3 | `StateChart` gives the golden flowchart for a state diagram with `class` lines. It keeps the states and edges, and colors each state and edge by its mark. | `go test ./internal/plan/infrastructure/mermaid -run TestStateChart -count=1` |
| 10.4 | `StateChart` keeps the `classDef` and `class` lines of a diagram. A class, ER or sequence diagram passes to the page unchanged, because `harness diagram` already marks it. | `go test ./internal/plan/infrastructure/mermaid -run TestStateChartKeepsMarks -count=1` |
| 10.5 | `RenderSVG` gives the golden SVG for a scene with each element kind. Two renders of the same scene are equal byte for byte. | `go test ./internal/plan/infrastructure/excalidraw -run TestRenderSVG -count=1` |
| 10.6 | An element with `customData.change` gets the change stroke. An unknown element kind returns `ErrUnknownElement` with the element id. | `go test ./internal/plan/infrastructure/excalidraw -run TestRenderSVGChange -count=1` |
| 10.7 | The page has no `src`, `href`, `url(` or `import` that points outside the page. | `go test ./internal/plan/infrastructure/view -run TestPageSelfContained -count=1` |
| 10.8 | Each criterion, task, spec section, tree file and commit has an anchor, and each in-page link points to an anchor that exists. | `go test ./internal/plan/infrastructure/view -run TestPageAnchors -count=1` |
| 10.9 | The page shows each check problem with its rule and a link to its source line. A clean plan shows no problem. | `go test ./internal/plan/infrastructure/view -run TestPageProblems -count=1` |
| 10.10 | The matrix and the trace panel show the state of each proof, file and commit, from the `gitdiff` adapter and the `ProofReader` port. | `go test ./internal/plan/infrastructure/view -run TestPageVerifyState -count=1` |
| 10.11 | With no branch and no evidence, each proof shows `not run`, each file shows its plan mark, and the page still renders. | `go test ./internal/plan/infrastructure/view -run TestPageNoEvidence -count=1` |
| 10.12 | The navigation has the groups Product, Tech specs, Tasks, QA and verification, and Signals, in this order. Each section is in one group. | `go test ./internal/plan/infrastructure/view -run TestPageNavGroups -count=1` |
| 10.13 | `harness plan view` writes the page in each phase and with no run, also for a plan with problems. An unreadable plan exits 1. | `go test ./internal/plan/ui/cli -run TestPlanViewCommand -count=1` |
| 10.14 | The view plugin renders the page when the new phase is `awaiting_approval`, and does nothing for another phase. | `go test ./internal/plan/ui/viewplugin -run TestViewPlugin -count=1` |
| 10.15 | The render records one `plan.view` span with the problem count and the page size. | `go test ./internal/plan/application/command/renderview -run TestRenderViewSpan -count=1` |
| 10.16 | Each ADR from 0001 to 0010 exists and has the sections Status, Context, Decision and Consequences. | `go test . -run TestADRSections -count=1` |
| 10.17 | The module is formatted, builds, vets and passes all tests with no error. | `make check` |

Tasks:

1. **Diagram marks and diagram rule.** Owned: `internal/shared/domain/diagram_marks.go`, `internal/plan/domain/rule.go`, `internal/plan/domain/check.go` and their tests, `internal/plan/domain/testdata/`. Test first: `diagram_marks_test.go` with `TestParseMarks`, which fails because `ParseMarks` does not exist. DoD: criteria 10.1 and 10.2. Depends on: none.
2. **State flowchart.** Owned: `internal/plan/infrastructure/mermaid/`. Test first: `flowchart_test.go` with `TestStateChart`, which fails because `StateChart` does not exist. DoD: criteria 10.3 and 10.4. Depends on: task 1.
3. **Excalidraw renderer.** Owned: `internal/plan/infrastructure/excalidraw/`, `./go.mod`, `./go.sum`. Test first: `render_test.go` with `TestRenderSVG`, which fails because the package does not exist. DoD: criteria 10.5 and 10.6. Depends on: task 1.
4. **Page renderer.** Owned: `internal/plan/domain/view.go`, `internal/plan/infrastructure/view/`. Test first: `page_test.go` with `TestPageSelfContained`, which fails because the renderer does not exist. DoD: criteria 10.7 to 10.12. Depends on: tasks 2 and 3.
5. **Command, plugin, CLI and decision record.** Owned: `internal/plan/application/command/renderview/`, `internal/plan/ui/cli/view.go`, `internal/plan/ui/cli/view_test.go`, `internal/plan/ui/viewplugin/`, `internal/plan/module.go`, `docs/adr/0010-plan-view-self-contained-html.md`, `./adr_test.go`, `./DESIGN.md`. Test first: `view_test.go` with `TestPlanViewCommand`, which fails because the command does not exist. DoD: criteria 10.13 to 10.17. Depends on: task 4.

Commits:

1. `feat: read the diagram marks and check the required diagrams` (task 1)
2. `feat: draw the marked states as a flowchart` (task 2)
3. `feat: render excalidraw scenes to SVG in Go` (task 3)
4. `feat: render the plan as a self-contained page` (task 4)
5. `feat: add harness plan view and render it for each approval` (task 5)

Code tree:

```tree
harness/
├── DESIGN.md                                       changed
├── adr_test.go                                     changed
├── go.mod                                          changed  github.com/dop251/goja
├── go.sum                                          changed
├── docs/
│   └── adr/
│       └── 0010-plan-view-self-contained-html.md   new
└── internal/
    ├── shared/
    │   └── domain/
    │       ├── diagram_marks.go                    changed
    │       └── diagram_marks_test.go               changed
    └── plan/
        ├── module.go                               changed
        ├── domain/
        │   ├── check.go                            changed
        │   ├── check_test.go                       changed
        │   ├── rule.go                             changed
        │   ├── view.go                             new
        │   └── testdata/
        │       └── diagrams-missing.md             new
        ├── application/
        │   └── command/
        │       └── renderview/
        │           ├── command.go                  new
        │           ├── handler.go                  new
        │           └── handler_test.go             new
        ├── infrastructure/
        │   ├── excalidraw/
        │   │   ├── render.go                       new
        │   │   ├── render_test.go                  new
        │   │   ├── rough.js                        new      rough.js 4.6.6, MIT
        │   │   └── testdata/
        │   │       ├── kinds.excalidraw            new
        │   │       └── kinds.svg.golden            new
        │   ├── mermaid/
        │   │   ├── flowchart.go                    new
        │   │   ├── flowchart_test.go               new
        │   │   └── testdata/
        │   │       ├── marked-state.mmd            new
        │   │       └── marked-state.golden         new
        │   └── view/
        │       ├── page.go                         new
        │       ├── page_test.go                    new
        │       ├── page.html.tmpl                  new
        │       ├── view.css                        new
        │       ├── view.js                         new
        │       ├── NOTICE.md                       new      mermaid MIT, rough.js MIT, Excalifont OFL 1.1
        │       └── assets/
        │           ├── mermaid.min.js              new      mermaid 11.17.2
        │           └── Excalifont.woff2            new
        └── ui/
            ├── cli/
            │   ├── view.go                         new
            │   └── view_test.go                    new
            └── viewplugin/
                ├── plugin.go                       new
                └── plugin_test.go                  new
```
