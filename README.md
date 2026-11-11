# harness

The harness is an open source Go tool and a plugin for Claude Code and Codex. It drives an AI coding agent through a fixed loop: plan, work, review, ship and watch production. You get a plan that waits for your approval. The agent writes the tests before the code. You also get gates with evidence, reviews on two or more models and a guard. It has five parts:

- **Run loop.** A run moves through fixed phases. Each move is one row of a transition table, and a plugin can veto it. `harness run` drives the run.
- **Gates.** A gate is one shell command that proves one property of the repo, such as the tests. `harness verify` runs the gates and the acceptance criteria of the plan, and keeps the evidence.
- **Lint packs.** Rule packs for DDD, event sourcing, observability, Next.js and shadcn, on Biome, oxlint and a built-in engine, with a baseline ratchet.
- **Guard.** Built-in rules on the host hooks. They deny a PR merge, a PR that is not a draft, a forced push, a read of a secret, a push before a commit check, and a stop while the run holds the turn.
- **Exec plugins.** Any command that reads a JSON event on stdin and prints a verdict. A plugin can veto a transition, a plan or a gate, propose gates, add lint findings, report a signal breach, or add context for the agent.

Status: pre-alpha.

## Quick start

1. Log in to GitHub, because the repository is private: run `gh auth login`, then `gh auth setup-git`.
2. Install the plugin in Claude Code: run `/plugin marketplace add 90p-labs/harness`, then `/plugin install 90p@harness`. Then run `/reload-plugins` or start a new session.
3. Build `harness` from source with Go 1.27, as in [Build from source](#build-from-source). Put `$(go env GOPATH)/bin` on the `PATH`.
4. In your repository, run `harness gates init`. Add `.harness/` to `.gitignore`. Commit `.harness.yaml` and `.gitignore`.
5. Start a run with `/90p:harness <prompt>`.

```console
$ harness gates init
go-fmt: test -z "$(gofmt -l .)"
go-vet: go vet ./...
go-test: go test ./...
```

With no detected toolchain, `harness gates init` writes no `.harness.yaml`. Write the `gates` key by hand, as in [docs/setup.md](docs/setup.md) and [examples/](examples/). Codex has [its own install steps](#codex).

## Install

The harness is a plugin for Claude Code and for Codex. The plugin installs these parts:

- The skills `harness` (the loop), `pr-open` (the pull request of the ship step), `lint-setup` (the architecture rules of a project), `ddd` (the DDD layout that the lint packs check), `demo` (a watched browser walkthrough) and `report` (a bug about the harness).
- The agents are `explorer` (the facts for a plan), `advisor` (the check of a plan) and `worker` (one task of a plan, or a fix). The others are `reviewer` (one lens of a run review), `verifier` (the findings of a round) and `linter` (the lint setup).
- The hooks of the host.

Each skill starts with a decision tree, so an agent reads only the part that it needs. The `pr-open` skill opens the pull request of the ship step. Its body answers Why, What and How, in this order, after a bottom line, with the plan diagrams and the visuals of the done tasks in How. The hooks act only in a git repository with `.harness.yaml` at its top level. In any other folder, they do nothing.

The repository is private. The GitHub login of quick start step 1 lets the host clone the plugin and the plugin download the binary.

The plugin puts no `harness` command on the `PATH`. See [the plugin binary](docs/setup.md#the-plugin-binary) for the download and build rules. Do not also run `harness host install` in a repository where you use the plugin: each hook then runs twice.

### Claude Code

```text
/plugin marketplace add 90p-labs/harness
/plugin install 90p@harness
```

From a shell, the same steps are `claude plugin marketplace add 90p-labs/harness` and `claude plugin install 90p@harness`. Then run `/reload-plugins` or start a new session. Start a run with `/90p:harness <prompt>`. To move from the old plugin name, see [the move](docs/setup.md#move-from-the-old-plugin-name).

### Codex

```sh
codex plugin marketplace add 90p-labs/harness
codex plugin add 90p@harness
```

Open `/hooks` in Codex once and trust the harness hooks. Codex does not run a plugin hook that you did not trust. Start a run with `$90p:harness <prompt>`.

### Build from source

The harness needs Go 1.27. Use this path for a repository without the plugin. [docs/setup.md](docs/setup.md#build-from-source) has the full block.

```sh
git clone https://github.com/90p-labs/harness.git
cd harness
go install ./cmd/harness
```

## Set up a repository

Run `harness gates init` to detect the gates and write the `gates` key of `.harness.yaml`. Use it with the plugin: the plugin installs the hooks. The lint packs call Biome and oxlint. Put them on the `PATH` with `npm i -g @biomejs/biome@2.6.6 oxlint@1.92.0`.

Without the plugin, `harness init --host claude` (or `--host codex`) sets up the repository in one step. It writes the gates, the lint files and the lint baseline, and installs the hooks. Do not use it with the plugin: each hook then runs twice. On a NestJS repository with the scripts `typecheck` and `test`:

```console
$ harness init --host claude
lint packs: wrote ddd
gates: wrote node-typecheck, node-test, lint
lint init: wrote .harness-lint
lint baseline: wrote .harness-lint/baseline.json (3 findings, run harness lint to see them)
host: wrote /home/dev/shop/.claude/settings.json
```

Commit the host config .claude/settings.json (or `.codex/hooks.json`) with `.harness.yaml` and `.harness-lint/`, so each clone gets the hooks. [docs/setup.md](docs/setup.md#step-by-step) gives the step by step setup, the tool table and the full config example. [`docs/configuration.md`](docs/configuration.md) explains every config key. The [full configuration](docs/configuration-full.md) page shows one example for each case.

## Your first run

A run has five steps:

1. Plan: the agent writes the plan.
2. Approve: you approve the plan.
3. Work: the agent writes the tests first, then the code.
4. Review: two or more reviewer models check the change.
5. Ship: the agent opens the pull request.

`harness plan view` shows the plan as one page, with the proof state of each criterion, the diagrams and the tasks:

![The plan summary with the acceptance criteria and their proof state](docs/images/plan-view-summary.png)

![The tech specs: a marked flowchart, a stateDiagram-v2 and a classDiagram](docs/images/plan-view-specs.png)

![The files, the tasks, the commits and the traceability matrix](docs/images/plan-view-tasks.png)

The agent asks the `Plan` question with the options `Approve` and `Change the plan`. A model that you name approves with `harness run approve --by <model> --reason "<one line>"`.

```console
$ harness run start "Let a customer cancel an order before it ships"
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=planning review_round=0 live_round=0
$ harness run plan
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=awaiting_approval review_round=0 live_round=0
$ harness run approve
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=working review_round=0 live_round=0
$ harness verify
PASS gates=1 cached=0
$ harness run review
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=reviewing review_round=0 live_round=0
$ harness run ship
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=shipping review_round=0 live_round=0
$ harness run done --pr https://github.com/acme/shop/pull/42
run 20261108-132848-let-a-customer-cancel-an-order-before-it phase=done review_round=0 live_round=0
```

The agent normally runs these commands, and the hooks enforce the order. The block leaves out some lines of the real output. [docs/runs.md](docs/runs.md) shows the full run with its plan, its output and the run phases.

## Everyday commands

| Command | What it does |
|---|---|
| `/90p:harness <prompt>` | Start a run |
| `/90p:harness status` | Show the run |
| `/90p:harness approve` | Approve the plan |
| `/90p:harness abandon` | Abandon the run |
| `harness run status` | Print the phase of the run |
| `harness run report` | Print the rework rounds, the stop blocks and the trace ID of the run |
| `harness plan view` | Write the plan as an HTML page |
| `harness verify` | Run the gates and the acceptance criteria |
| `harness verify --commits main` | Check each commit of the branch |
| `harness pr body`, `harness pr check --body-file <path>` | Print a PR body skeleton, or every problem of a PR body |
| `harness lint` | Run the lint packs |
| `harness config` | Print the effective config |
| `/90p:report` | File a bug about the harness |

## Examples

- [`examples/symfony/`](examples/symfony/README.md): a Makefile with `## setup` and `## gate` targets and a `gates.detect` plugin.
- [`examples/go/`](examples/go/README.md): the `setup` and `gates` keys, with the gate ids of the built-in detection of `go.mod`.
- [`examples/nextjs/`](examples/nextjs/README.md): the `setup` and `gates` keys and the lint packs `nextjs` and `shadcn`.
- [`examples/python/`](examples/python/README.md): the `setup` and `gates` keys, and custom lint packs with ruff and import-linter plugins.
- [`examples/README.md`](examples/README.md): the four projects side by side, with the gates in containers.

## Documents

- [`docs/setup.md`](docs/setup.md): the plugin binary, the lint tools, `harness init` and the step by step setup.
- [`docs/plugins.md`](docs/plugins.md): the plugin contract and the five plugin kinds.
- [`docs/runs.md`](docs/runs.md): a full run, the plan view and the run phases.
- [`docs/contributing.md`](docs/contributing.md): the repository layout, the tests and the bug report.
- [`docs/configuration.md`](docs/configuration.md): every key of `.harness.yaml` and how to configure it.
- [`docs/configuration-full.md`](docs/configuration-full.md): a full `.harness.yaml` and one example for each case.
- [`docs/go-live.md`](docs/go-live.md): follow a run from the merge to production.
- [`docs/adr/`](docs/adr/): the architecture decision records.
- [`DESIGN.md`](DESIGN.md): the modules, the loop, the events, the config keys and the rules.
- [`CONTEXT.md`](CONTEXT.md): the terms.
- [`ROADMAP.md`](ROADMAP.md): the pull request that builds each module.

## License

MIT. See `LICENSE`.
