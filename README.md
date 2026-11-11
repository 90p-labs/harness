# harness

The harness is an open source Go tool and a plugin for Claude Code and Codex. It drives an AI coding agent through a fixed loop: plan, approve, work, review, ship and watch production.

Status: pre-alpha. The repository is private, so log in to GitHub first: run `gh auth login`, then `gh auth setup-git`.

## Install

The harness is a plugin for Claude Code and for Codex. The plugin installs these parts:

- The skills `harness` (the loop), `pr-open` (the pull request of the ship step), `lint-setup` (the architecture rules of a project), `ddd` (the DDD layout that the lint packs check), `demo` (a watched browser walkthrough) and `report` (a bug about the harness).
- The agents are `explorer` (the facts for a plan), `advisor` (the check of a plan) and `worker` (one task of a plan, or a fix). The others are `reviewer` (one lens of a run review), `verifier` (the findings of a round) and `linter` (the lint setup).
- The hooks of the host. They act only in a git repository with `.harness.yaml` at its top level.

The plugin puts no `harness` command on the `PATH`, so also build it from source. See [the plugin binary](docs/setup.md#the-plugin-binary) for the download and build rules.

### Claude Code

```text
/plugin marketplace add 90p-labs/harness
/plugin install 90p@harness
```

From a shell, run `claude plugin marketplace add 90p-labs/harness` and `claude plugin install 90p@harness`. Then run `/reload-plugins` or start a new session. To move from the old plugin name, see [the move](docs/setup.md#move-from-the-old-plugin-name).

### Codex

```sh
codex plugin marketplace add 90p-labs/harness
codex plugin add 90p@harness
```

Open `/hooks` in Codex once and trust the harness hooks. Codex does not run a plugin hook that you did not trust.

### Build from source

The harness needs Go 1.27. Put `$(go env GOPATH)/bin` on the `PATH`. [docs/setup.md](docs/setup.md#build-from-source) has the full block.

```sh
git clone https://github.com/90p-labs/harness.git
cd harness
go install ./cmd/harness
```

## Set up a repository

In your repository, run `harness gates init`. It detects the gates and writes the `gates` key of `.harness.yaml`. Add `.harness/` to `.gitignore`, then commit both files.

```console
$ harness gates init
go-fmt: test -z "$(gofmt -l .)"
go-vet: go vet ./...
go-test: go test ./...
```

With no detected toolchain, write the `gates` key by hand, as in [docs/setup.md](docs/setup.md) and [examples/](examples/). The lint packs call Biome and oxlint: `npm i -g @biomejs/biome@2.6.6 oxlint@1.92.0`.

Without the plugin, `harness init --host claude` (or `--host codex`) writes the gates, the lint files and the baseline, and installs the hooks. Do not use it with the plugin, or each hook runs twice. [`docs/configuration.md`](docs/configuration.md) explains every config key.

## Usage

Start a run in Claude Code with `/90p:harness <prompt>`, or in Codex with `$90p:harness <prompt>`. The agent then drives the loop, and you answer its questions.

### Your first run

```text
/90p:harness Let a customer cancel an order before it ships
```

The agent writes the plan, and `harness plan view` shows it as one page. The page shows the phase, the state of each task, the proof of each criterion, the diagrams and the tasks. It updates on every phase change:

![The plan summary: the goal, the acceptance criteria with their proof state, and the properties pane](docs/images/plan-view-summary.png)

![The tech specs with their criteria, the change colors and a layout sketch](docs/images/plan-view-specs.png)

![The task board: one column for each wave, with the test first, the files, the DoD and the commit](docs/images/plan-view-tasks.png)

The agent asks the `Plan` question with the options `Approve` and `Change the plan`. A model that you name approves with `harness run approve --by <model> --reason "<one line>"`. Under the hood, the agent runs these commands, and the hooks enforce the order:

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

[docs/runs.md](docs/runs.md) shows the full run with its plan and its output.

### Everyday commands

| Command | What it does |
|---|---|
| `/90p:harness <prompt>` | Start a run |
| `/90p:harness status`, `approve`, `abandon` | Show the run, approve the plan, or abandon the run |
| `harness run status`, `harness run report` | Print the phase, or the rework rounds, the stop blocks and the trace ID |
| `harness plan view` | Write the plan as an HTML page |
| `harness verify` | Run the gates and the acceptance criteria |
| `harness verify --commits main` | Check each commit of the branch |
| `harness pr body`, `harness pr check --body-file <path>` | Print a PR body skeleton, or every problem of a PR body |
| `harness lint`, `harness config` | Run the lint packs, or print the effective config |
| `/90p:report` | File a bug about the harness |

### Example projects

- [`examples/go/`](examples/go/README.md): the `setup` and `gates` keys, with the gate ids of the built-in detection of `go.mod`.
- [`examples/nextjs/`](examples/nextjs/README.md): the `setup` and `gates` keys and the lint packs `nextjs` and `shadcn`.
- [`examples/python/`](examples/python/README.md): custom lint packs with ruff and import-linter plugins.
- [`examples/symfony/`](examples/symfony/README.md): a Makefile with `## setup` and `## gate` targets and a `gates.detect` plugin.
- [`examples/README.md`](examples/README.md): the four projects side by side, with the gates in containers.

## Goals

An agent should not merge what you did not approve, or claim what it did not prove. Each goal maps to one part of the harness:

- **You approve the plan first.** The plan holds the goal, the criteria, the diagrams and the tasks. The agent starts work only after your answer.
- **Proof, not claims.** A gate is one shell command that proves one property, such as the tests. `harness verify` runs the gates and each criterion, and keeps the evidence.
- **Tests before code.** A `test:` commit must fail a gate, and every other commit must pass.
- **More than one reviewer.** Two or more models review the change, and a verifier checks each finding.
- **Hard limits.** The guard rules run on the host hooks. They deny a PR merge, a PR that is not a draft, a forced push, a read of a secret and an early stop.
- **Clean architecture.** Lint packs for DDD, event sourcing, observability, Next.js and shadcn, with a baseline ratchet.
- **Open to your tools.** An exec plugin reads a JSON event on stdin and prints a verdict. It can veto a transition, a plan or a gate.

## Phases

A run moves through fixed phases. Each move is one row of a transition table in `internal/run/domain`, and a plugin can veto it.

| Phase | What happens | Next |
|---|---|---|
| `planning` | The explorer reads the code, the agent writes `plan.md`, and the advisor checks it | `harness run plan` |
| `awaiting_approval` | The agent asks you the open questions, then the `Plan` question | `harness run approve` |
| `working` | Workers write the tests first, then the code, and `harness verify` runs the gates | `harness run review` |
| `reviewing` | The reviewers check the change and a verifier checks each finding | `harness run ship` or `harness run rework` |
| `shipping` | The agent opens a draft pull request, and you merge it | `harness run done --pr URL` |
| `releasing`, `observing` | With go-live on, the run follows the deploy and the signals | `harness live check` |
| `done` | The run is closed | `harness run start` |

A run can also go to `blocked` (for example after the last review round) or `abandoned`. [docs/runs.md](docs/runs.md#run-phases) has the full state diagram, and [docs/go-live.md](docs/go-live.md) covers the release phases.

## Documents

- [`docs/setup.md`](docs/setup.md): the plugin binary, the lint tools, `harness init` and the step by step setup.
- [`docs/configuration.md`](docs/configuration.md) and [`docs/configuration-full.md`](docs/configuration-full.md): every key of `.harness.yaml`, and one example for each case.
- [`docs/runs.md`](docs/runs.md): a full run, the plan view, the multi-run plan and the run phases.
- [`docs/plugins.md`](docs/plugins.md): the plugin contract and the five plugin kinds.
- [`docs/go-live.md`](docs/go-live.md): follow a run from the merge to production.
- [`docs/contributing.md`](docs/contributing.md): the repository layout, the tests and the bug report.
- [`DESIGN.md`](DESIGN.md), [`CONTEXT.md`](CONTEXT.md), [`ROADMAP.md`](ROADMAP.md) and [`docs/adr/`](docs/adr/): the design, the terms, the build order and the decisions.

## License

MIT. See `LICENSE`.
