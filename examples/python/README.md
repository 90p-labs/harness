# Python example

This example shows how a Python project with uv sets up the harness.
It shows two patterns that the other examples do not show:

- Gates that you write by hand in the `gates` key. The harness has no Python detection, and no plugin writes the gates.
- A custom lint pack with a lint engine plugin. The plugin runs ruff in the container and returns its diagnostics as findings.

It uses a `setup` step and `needs`, as the Go example does.
It shows the architecture rules of the `lint-setup` skill with import-linter.
The gates, ruff and import-linter run in a Docker Compose service. The harness, the hooks and the plugin scripts run on the host.

## Files

The example holds only the files that you add to your project:

| File | Purpose |
|---|---|
| `compose.yaml` | The service `python`. It uses the pinned `uv` image and keeps the venv in a named volume. |
| `.harness.yaml` | The `ruff` and `import-linter` plugins, the two lint packs, `verify`, the `setup` step `py-install` and the three gates. |
| `lint/python/pack.yaml` | The custom lint pack `python`. Its one rule names the engine `ruff`. |
| `.harness/plugins/ruff-lint.sh` | The lint engine plugin of ruff. It answers `lint.check`. |
| `.importlinter` | The import-linter config: the contracts. You own this file. |
| `lint/architecture/pack.yaml` | The custom lint pack `architecture`. Its four rules name the engine `import-linter`. |
| `.harness/plugins/import-linter.sh` | The lint engine plugin of import-linter. It answers `lint.check`, runs `lint-imports` and maps each broken contract to a rule id of the catalogue. |
| `.harness/plugins/domain_purity.py` | A custom import-linter contract type. A domain may import only itself, the shared domain, the standard library and an allow list. |

## Prerequisites

1. Install Docker and Compose. The proof used Docker Engine 29.9.2 and Compose v5.6.1.
2. Put `harness`, `git`, `sh` and `jq` on the `PATH` of the host. The plugin scripts run on the host and use `jq`.
3. Use a uv project with `pytest`, `mypy`, `ruff` and `import-linter` in the `dev` group: `uv add --dev pytest mypy ruff import-linter`. Then `uv.lock` pins them.

The proof used `pytest` 9.2.0, `mypy` 2.4.1, `ruff` 0.17.4 and `import-linter` 2.16, from `uv init --lib`. The proof used jq 1.6.
The image is `ghcr.io/astral-sh/uv:0.13.7-python3.14-trixie-slim`. It holds uv 0.13.7 and Python 3.14.10.
The `requires-python` of the project must accept Python 3.14.

The project needs no Node, no Biome, no oxlint and no `lint.disabled`.
The harness calls a lint engine only when an active pack has a rule for that engine.
The pack `python` has rules only for the engine `ruff`, so the harness never starts Biome or oxlint.

## What runs where

- The harness, the hooks, the gate `lint` and the plugin scripts run on the host.
- The setup step `py-install`, the gates `py-test` and `py-typecheck`, the ruff check and the import-linter check run in the container with `docker compose run --rm -T python`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.
- The setup step `py-install` runs `uv sync --locked` before the gates. `--locked` stops when `uv.lock` does not match `pyproject.toml`. `uv sync` also removes each package that `uv.lock` does not hold. Without that, a package that a commit removed stays in the shared venv, and an import of it still passes.
- The gates run `uv run --no-sync`. They use the venv as the setup step left it, and they do not sync a second time. The container gates `py-typecheck` and `py-test` do not write to the venv.
- The ruff plugin and the import-linter plugin run `uv run --no-sync`. They read the venv and do not write to it. `harness lint` does not run the setup step, so run `docker compose run --rm -T python uv sync --locked` once before the first `harness lint`, and again after a dependency change. `harness verify` runs the setup step for you.
- The import-linter plugin sets `PYTHONDONTWRITEBYTECODE=1`, so the container writes no `__pycache__/` into the project. ruff does not import the project code, so the ruff plugin needs no such setting.
- Each plugin names its container. On a timeout, the harness sends `TERM` to the plugin, and the plugin stops its container with `docker kill`. A `TERM` to `docker compose run` stops only the Compose client, not the container.
- The temporary worktree of `harness verify --commits` uses the same venv volume. The setup step runs there when the hash of its `paths` differs from the hash of its last pass, and the sync puts the packages of the checked commit into the venv.
- The venv is in the named volume `venv`, at `UV_PROJECT_ENVIRONMENT=/opt/venv`. The project gets no `.venv/`, and the host holds no Linux binary.
- The uv cache is in the named volume `uv-cache`. The cache and the venv are on two file systems, so uv cannot hardlink. Without `UV_LINK_MODE=copy`, uv prints "Failed to hardlink files; falling back to full copy" on each install.
- `UV_PYTHON_DOWNLOADS=never` makes uv use the Python of the image. A `requires-python` that the image does not match then stops the gate, and uv downloads no other Python. With `requires-python = ">=3.15"`, uv exits with code 2: "The Python request from `.python-version` resolved to Python 3.14.10, which is incompatible with the project's Python requirement".

Set the top-level `name:` of `compose.yaml` to the name of your project.
`harness verify --commits` runs the gates in a temporary worktree. The fixed name lets that worktree use the same venv volume.
A second checkout that runs gates at the same time must export `COMPOSE_PROJECT_NAME` with a different value. Two syncs into one venv can break each other.
The harness keeps one setup pass for the repo, so it can skip the setup step in that checkout, and its new venv volume stays empty.
Run `docker compose run --rm -T python uv sync --locked` once in that checkout.
Put `compose.yaml` and `pyproject.toml` at the git root. The plugin compares the ruff paths with the git paths of the payload, so a project in a subfolder gets no finding.

## Pull the image

Pull the image before the first `harness verify`. A pull inside a gate counts against the gate timeout.
A first pull downloads 63 MB of compressed layers on `linux/amd64` and 62 MB on `linux/arm64`, as the registry manifest gives them.
The image is 262 MB on disk. The first pull on the test host took 3.3 s.
The pull below took 1.1 s, because the host already held the layers.

```console
$ time docker compose pull
 Image ghcr.io/astral-sh/uv:0.13.7-python3.14-trixie-slim@sha256:e80c05a310400cfc90af00b5b7271ef9ad24816e3265e1162af8a678e1aad099 Pulling 
 Image ghcr.io/astral-sh/uv:0.13.7-python3.14-trixie-slim@sha256:e80c05a310400cfc90af00b5b7271ef9ad24816e3265e1162af8a678e1aad099 Pulled 

real	0m1.114s
user	0m0.052s
sys	0m0.038s
```

## Steps

The proof started from a `uv init --lib` project with `tests/test_order.py`, `uv.lock`, `.gitignore` and the `compose.yaml` of this example, in one commit. The proof ran `uv init` and `uv add` in the container.
The project had three bounded contexts, and each folder is a package:

- `src/shop/order` with the folders `domain`, `application` and `infrastructure`.
- `src/shop/billing` with the folders `domain` and `application`.
- `src/shop/shared` with the folder `domain`. This is the shared kernel: `billing` uses its `Money` class.

Each Python file sits in a context and a layer. A file outside them, such as `src/shop/cart.py`, is a `ddd/src-roots` finding.
`src/shop/order/domain/order.py` had one unused import, `import os`.
The paths in the output below show `/path/to/app` for the project folder.

### 1. See that no detection runs

```console
$ harness gates init
$ cat .harness.yaml
cat: .harness.yaml: No such file or directory
```

`harness gates init` finds no toolchain that it knows, so it prints nothing and writes no `.harness.yaml`.

### 2. Copy the example

```console
$ cp /path/to/harness/examples/python/compose.yaml /path/to/harness/examples/python/.harness.yaml .
$ cp /path/to/harness/examples/python/.importlinter .
$ mkdir -p .harness/plugins lint/python lint/architecture
$ cp /path/to/harness/examples/python/.harness/plugins/ruff-lint.sh .harness/plugins/
$ cp /path/to/harness/examples/python/.harness/plugins/import-linter.sh .harness/plugins/
$ cp /path/to/harness/examples/python/.harness/plugins/domain_purity.py .harness/plugins/
$ cp /path/to/harness/examples/python/lint/python/pack.yaml lint/python/
$ cp /path/to/harness/examples/python/lint/architecture/pack.yaml lint/architecture/
```

Add these lines to `.gitignore`:

```
__pycache__/
.pytest_cache/
.mypy_cache/
.harness/*
!.harness/plugins/
```

The first three lines ignore the caches that the tools write into the project. Then a cache does not change the input hash of a gate.
`.harness/*` ignores the run state of the harness, and `!.harness/plugins/` keeps the plugins in git.

The `setup` key of `.harness.yaml` holds one step, and the `gates` key holds three gates. You write them by hand:

- `py-install` is the setup step. It runs `uv sync --locked` in the container.
- `py-typecheck` runs `uv run --no-sync mypy src tests` in the container. Put the mypy settings, such as `strict`, in the `[tool.mypy]` table of `pyproject.toml`.
- `lint` runs `harness lint --json` on the host, with the baseline `.harness-lint/baseline.json`.
- `py-test` runs `uv run --no-sync pytest` in the container. It has `needs: [py-typecheck, lint]`.

`verify.parallel: 2` lets the harness run two gates at the same time.
`py-typecheck` and `lint` need nothing, so they start together.
`py-test` starts only after `py-typecheck` and `lint` pass or are cached.
When one of them fails, the harness does not run `py-test`. It prints `SKIPPED py-test needs <gate>` and exits with code 1.
`harness verify` runs the setup step before the first gate, and only when the hash of its `paths` changed since its last pass. A step that runs prints `setup py-install <duration>`. A step that does not run prints nothing.

### 3. Write the lint files and see the finding

The pack `python` has one rule, `python/ruff`, with the engine `ruff` and the scope `**/*.py`.
The scope leaves out the `.pyi` stubs. Ruff checks them, but the plugin drops their diagnostics, because the payload does not hold them.
The `plugins` entry `ruff` answers `lint.check`, so the harness sends it the rules of the engine `ruff` and the git files that match the scope.
The plugin does these steps:

1. It reads the event from stdin before it starts the container. Then `docker compose run` cannot read the event.
2. It runs `ruff check --output-format json` on the project in the container. The `[tool.ruff]` table of `pyproject.toml` selects the ruff codes.
3. It gives the event and the ruff report to `jq` on stdin, not as arguments. A large file list can go above the argument limit of the system.
4. It keeps the diagnostics of the files in the payload. It removes the `/app/` prefix, because the harness refuses an absolute file.
5. It returns each diagnostic as a finding of the rule `python/ruff`. The message starts with the ruff code. A syntax error has no ruff code, so its message has only the ruff text.

The plugin passes `--exit-zero` to ruff, because a plugin that exits with a non-zero code is a plugin failure.
It passes `--no-cache`, so ruff writes no `.ruff_cache/` into the project.
It passes `--progress quiet` to Compose, because the harness copies the stderr of the plugin to its own stderr.

Set `timeout` in the `plugins` entry above the start time of the container. The default is 5 s.
The harness runs the engines one after the other, in the order of their names, so the import-linter plugin runs first.
The example sets `timeout: 60s`, so a slow start of the container does not fail the plugin.

The plugins do not sync the venv, so sync it once before the first `harness lint`. In the proof, `uv add` had already filled the venv, so the sync only checked it:

```console
$ harness lint init
$ cat .harness-lint/VERSION
dev+a7768ce0f35c
$ docker compose run --rm -T python uv sync --locked
Resolved 20 packages in 1ms
Checked 19 packages in 0.49ms
$ time harness lint
error python/ruff src/shop/order/domain/order.py:1 F401 `os` imported but unused
error: lint found error findings

real	0m1.343s
user	0m0.167s
sys	0m0.102s
$ time harness lint
error python/ruff src/shop/order/domain/order.py:1 F401 `os` imported but unused
error: lint found error findings

real	0m1.370s
user	0m0.166s
sys	0m0.103s
```

The listing of `docker compose run` leaves out the two `Container ... Creating` and `Created` lines that Compose prints.

`.harness-lint/VERSION` holds the binary version, a `+` and a hash of the custom pack files. Run `harness lint init` again after each edit of `lint/python/` or `lint/architecture/`.
The finding of the ruff pack is the only one, because the code has no broken contract.

### 4. Record the baseline and install the hooks

```console
$ harness lint baseline
$ cat .harness-lint/baseline.json
{
  "version": 1,
  "findings": [
    {
      "rule": "python/ruff",
      "severity": "error",
      "file": "src/shop/order/domain/order.py",
      "line": 1,
      "message": "F401 `os` imported but unused"
    }
  ]
}
$ harness host install --host claude
/path/to/app/.claude/settings.json
```

`harness lint` still exits with code 1 on this finding. Only the gate `lint` reads the baseline.
Commit `.harness.yaml`, `compose.yaml`, `.importlinter`, `.gitignore`, `.harness/plugins/`, `lint/`, `.harness-lint/` and `.claude/`.

### 5. Verify two times

```console
$ harness verify
setup py-install 273ms
PASS gates=3 cached=0
$ harness verify
PASS gates=3 cached=3
```

The first run starts with `setup py-install`, because the step never passed before. Then `py-typecheck` and `lint` run, two at a time, and `py-test` runs after the two.
The gate `lint` passes, because the baseline holds the finding.
The second run prints no `setup` line and shows `cached=3`, because no input changed.

### 6. See the ratchet stop a new finding

The proof added `import json` and an empty line at the top of `tests/test_order.py`:

```console
$ harness verify
FAIL lint 1.235s
SKIPPED py-test needs lint

lint:
{"version":1,"findings":[{"rule":"python/ruff","severity":"error","file":"src/shop/order/domain/order.py","line":1,"message":"F401 `os` imported but unused"},{"rule":"python/ruff","severity":"error","file":"tests/test_order.py","line":1,"message":"F401 `json` imported but unused"}]}

FAIL gates=3 failed=1
error: gates failed
$ echo $?
1
```

The gate `lint` fails on the finding in `tests/test_order.py`, because the baseline does not hold it.
`py-typecheck` passes and prints no line, because the harness prints a line only for a failed or skipped gate.
`py-test` needs `lint`, so the harness does not run it. It prints `SKIPPED py-test needs lint`, and `harness verify` exits with code 1.
The report also lists the finding in the baseline. The baseline key is the rule, the file and the message, not the line.
The proof then removed the new import. The next `harness verify` printed `cached=3`, because the files were the same as in step 5.

### 7. Change a dependency

The setup step runs again when `uv.lock` changes. The proof added a dev dependency in the container, with `--no-sync` so that only `uv.lock` and `pyproject.toml` change:

```console
$ docker compose run --rm -T python uv add --no-sync --dev pytest-timeout
Resolved 21 packages in 235ms
$ git diff --stat
 pyproject.toml |  1 +
 uv.lock        | 14 ++++++++++++++
 2 files changed, 15 insertions(+)
$ harness verify
setup py-install 435ms
PASS gates=3 cached=0
$ docker compose run --rm -T python uv pip list --python /opt/venv/bin/python | grep timeout
Using Python 3.14.10 environment at: /opt/venv
pytest-timeout    2.4.0
$ harness verify
PASS gates=3 cached=3
```

The first run prints `setup py-install` and installs `pytest-timeout` into the venv. The second run, with no change, prints no `setup` line.
The proof then removed the dependency. The files returned to the state before the new dependency:

```console
$ docker compose run --rm -T python uv remove --no-sync --dev pytest-timeout
Resolved 20 packages in 12ms
$ harness verify
setup py-install 288ms
PASS gates=3 cached=3
$ docker compose run --rm -T python uv pip list --python /opt/venv/bin/python | grep -c timeout
Using Python 3.14.10 environment at: /opt/venv
0
```

The setup step ran again, because the hash of `uv.lock` changed. It removed `pytest-timeout` from the venv, because `uv sync` removes each package that `uv.lock` does not hold.
The gates are cached, because their input hash is the same as before the new dependency.

### 8. Fix the old finding and shrink the baseline

The proof removed `import os` from `src/shop/order/domain/order.py`:

```console
$ harness lint
$ harness lint baseline
$ cat .harness-lint/baseline.json
{
  "version": 1,
  "findings": []
}
```

`harness lint baseline` only removes the findings that no longer occur, so the baseline only shrinks.
Commit the fix and the baseline. `harness verify --commits` checks the commit in a temporary worktree:

```console
$ harness verify --commits HEAD~1
PASS commits=1
```

The setup step prints no line here, because the hash of its `paths` is the same as the hash of its last pass.

## Architecture rules with import-linter

The `lint-setup` skill wrote the config, the plugin and the pack with its steps. `harness lint catalogue` prints the rules of the catalogue.
The project does not turn on the built-in `ddd` pack, because its engine reads TypeScript.
The custom pack `architecture` gives four rules of the `ddd` pack to the engine `import-linter`. Its scope is `src/**/*.py`.
The one waived rule is `ddd/file-suffix`. import-linter checks the imports, not the names of the files, so the custom pack has no rule for it.
`.importlinter` sets `root_package = shop`, so import-linter reads only `src/shop`.

The skill proved the rules once at the setup. It wrote a temporary proof folder `lint/proof/` with one canary for each rule, and it ran `harness lint canaries --proof lint/proof`. Each canary breaks its rule on purpose.
The skill reported each PASS and each WAIVED line with its reason to the user, and then it deleted the folder.
The project commits no canary, no manifest and no canary gate. A later edit of `.importlinter` that lets a rule through stays green until someone runs the `lint-setup` skill again.

`.importlinter` has six contracts. The `name` of a contract is the rule id, so the plugin needs no table. Two rules have two contracts each:

- `ddd/src-roots` has two `layers` contracts with `exhaustive = true`. The first lists the contexts in `shop`: `(order) : (billing) : (shared)`. The second lists the layers in each context: `(infrastructure) : (application) : (domain)`. A module that a contract does not list, such as `shop.cart`, `shop.payment` or `shop.order.utils`, is a finding. Add each new context to both lists of contexts.
- `ddd/domain-purity` uses the contract type `domain_purity` from `.harness/plugins/domain_purity.py`. Each `shop.*.domain` may import only itself, the modules in `allowed_modules` and the standard library of Python. The example allows `shop.shared.domain`. Add a library that the domain may use, such as `pydantic`, to `allowed_modules`. `include_external_packages = True` makes import-linter see the packages that are not in `shop`. The package does not need to be installed.
- `ddd/layer-direction` is a `layers` contract. The containers `shop.*` each hold the layers `(infrastructure)` and `(application)`. The brackets make a layer optional, because `shop.shared` has only a domain.
- `ddd/cross-context` has an `independence` contract on the modules `shop.*`. Its `ignore_imports` let each context use `shop.shared` and the package of another context, which is its public entry point, but not an inner module. `unmatched_ignore_imports_alerting = none` stops an error, because the project has no such import.
- `ddd/cross-context` also has a `layers` contract on `shop`: `(order) : (billing)` above `(shared)`. The shared kernel `shop.shared` imports no context.

In a layers contract, `:` puts two layers side by side with no import rule between them. A context import of another context is the job of the independence contract, so it gives one finding, not two.
A domain import of the application layer, the infrastructure layer or another context gives the `ddd/domain-purity` finding. An import of another context also gives `ddd/cross-context`, so one import can give two rule ids.
Edit the contracts to match your project. Then run the `lint-setup` skill again to prove the rules.

The plugin `import-linter.sh` does these steps:

1. It reads the event from stdin before it starts the container.
2. It reads the folder of the call with `git rev-parse --show-prefix`. The harness runs the plugin in a canary folder for a proof, and in the project root for `harness lint`.
3. It runs `lint-imports` in the container with `PYTHONPATH` set to `/app/<folder>src:/app/.harness/plugins`. The package `shop` of a proof canary then comes before the package `shop` of the project, and import-linter finds the contract type `domain_purity`. The config is `/app/.importlinter` for each call.
4. It sets `COLUMNS=1000`. Without a terminal, the report wraps at 80 columns and splits an import line in two.
5. It looks for the summary line `Contracts: <n> kept, <m> broken.` Without it, the config is not valid and there is no report. The plugin prints the tool text to stderr and exits with 1. A broken contract is a finding, not a plugin failure.
6. It checks that each contract name is a rule id, such as `ddd/cross-context`. A contract with another name is a plugin error that names the contract, so a contract never takes the rule id of the contract above it.
7. It reads the report with `awk`. It keeps each direct import. It drops a chain that goes through another module when a later link of the chain is a direct import of a rule in the event. That import already gives a finding, under the rule of its own contract. Else the first import of the chain gives the finding, also each extra first import that import-linter adds with `&`. An unlisted module of an exhaustive contract gives a finding on the first line of its file.
8. It gives the event and the imports to `jq` on stdin. `jq` maps a module to `src/<path>.py` or `src/<path>/__init__.py`, and keeps the file that is in the payload. It keeps only the rules of the event, and one finding for each rule, file and line.

A domain class that uses a library gives a finding. The proof added `src/shop/order/domain/clock.py` with `import requests`:

```console
$ harness lint
error ddd/domain-purity src/shop/order/domain/clock.py:1 shop.order.domain is not allowed to import requests: shop.order.domain.clock -> requests
error python/ruff src/shop/order/domain/order.py:1 F401 `os` imported but unused
error: lint found error findings
$ echo $?
1
```

The ruff finding still shows. The proof then removed the file.

The shared kernel gives no finding: `src/shop/billing/domain/invoice.py` imports `shop.shared.domain.money`, and `harness lint` gave only the ruff finding.
A shared file that imports a context gives two findings. The proof added `src/shop/shared/domain/bad.py` with `from shop.order.domain.order import Order`:

```console
$ harness lint
error python/ruff src/shop/order/domain/order.py:1 F401 `os` imported but unused
error ddd/cross-context src/shop/shared/domain/bad.py:1 shop.shared is not allowed to import shop.order: shop.shared.domain.bad -> shop.order.domain.order
error ddd/domain-purity src/shop/shared/domain/bad.py:1 shop.shared.domain is not allowed to import shop.order.domain.order: shop.shared.domain.bad -> shop.order.domain.order
error: lint found error findings
```

An import from one context into another gives one finding. The proof added `src/shop/billing/application/order_total.py` with `from shop.order.domain.order import Order`:

```console
$ harness lint
error ddd/cross-context src/shop/billing/application/order_total.py:1 shop.billing is not allowed to import shop.order: shop.billing.application.order_total -> shop.order.domain.order
error python/ruff src/shop/order/domain/order.py:1 F401 `os` imported but unused
error: lint found error findings
```

A chain gives no extra finding when a later link is a direct import that breaks a contract. That import gives the finding, under its own rule. The proof made `src/shop/billing/domain/invoice.py` import `shop.billing.infrastructure.printer`. `src/shop/billing/application/issue_invoice.py` imports the domain, so the application reaches the infrastructure through the domain:

```console
$ harness lint
error ddd/domain-purity src/shop/billing/domain/invoice.py:3 shop.billing.domain is not allowed to import shop.billing.infrastructure.printer: shop.billing.domain.invoice -> shop.billing.infrastructure.printer
error python/ruff src/shop/order/domain/order.py:1 F401 `os` imported but unused
error: lint found error findings
```

A chain with no direct break gives a finding on each file that starts it. The proof made `src/shop/order/__init__.py` import `shop.order.infrastructure.memory_repository`. It added `wire.py` and `rewire.py` in `src/shop/order/application/`, each with `from shop.order import MemoryOrderRepository`. The entry point `shop.order` is in no layer, so only the chain breaks `ddd/layer-direction`:

```console
$ harness lint
error ddd/layer-direction src/shop/order/application/rewire.py:1 shop.order.application is not allowed to import shop.order.infrastructure: shop.order.application.rewire
error ddd/layer-direction src/shop/order/application/wire.py:1 shop.order.application is not allowed to import shop.order.infrastructure: shop.order.application.wire -> shop.order
error python/ruff src/shop/order/domain/order.py:1 F401 `os` imported but unused
error: lint found error findings
```

A file outside the listed contexts and layers gives `ddd/src-roots`. The proof added `src/shop/cart.py`, `src/shop/payment/__init__.py` and `src/shop/order/utils.py`:

```console
$ harness lint
error ddd/src-roots src/shop/cart.py:1 shop.cart is in no context and layer that .importlinter lists
error python/ruff src/shop/order/domain/order.py:1 F401 `os` imported but unused
error ddd/src-roots src/shop/order/utils.py:1 shop.order.utils is in no context and layer that .importlinter lists
error ddd/src-roots src/shop/payment/__init__.py:1 shop.payment is in no context and layer that .importlinter lists
error: lint found error findings
```

A contract of your own needs a rule id as its name. The proof added a `forbidden` contract with `name = No pandas` after the `ddd` contracts:

```console
$ harness lint
import-linter.sh: a contract name must be a rule id, such as ddd/cross-context. These names are not:
No pandas
error: lint engine import-linter: plugin failed: import-linter: exit status 1
```

Give the contract the id of the rule that it checks, or put it in a config of its own that this plugin does not read. The proof removed each file and contract after its check.

## The project after the setup

The marks on the right are not part of a `tree` output.
The project keeps no `lint/proof/` folder, because the skill deletes it after the proof. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.

```text
.
├── .claude
│   └── settings.json        harness writes (host install), commit
├── .harness
│   └── plugins
│       ├── domain_purity.py     you add, commit
│       ├── import-linter.sh     you add, commit
│       └── ruff-lint.sh         you add, commit
├── .harness-lint
│   ├── VERSION              harness writes (lint init), commit
│   └── baseline.json        harness writes (lint baseline), commit
├── lint
│   ├── architecture
│   │   └── pack.yaml        you add, commit
│   └── python
│       └── pack.yaml        you add, commit
├── src
│   └── shop
│       ├── billing
│       │   ├── application
│       │   │   ├── __init__.py
│       │   │   └── issue_invoice.py
│       │   ├── domain
│       │   │   ├── __init__.py
│       │   │   └── invoice.py
│       │   └── __init__.py
│       ├── order
│       │   ├── application
│       │   │   ├── __init__.py
│       │   │   └── place_order.py
│       │   ├── domain
│       │   │   ├── __init__.py
│       │   │   └── order.py
│       │   ├── infrastructure
│       │   │   ├── __init__.py
│       │   │   └── memory_repository.py
│       │   └── __init__.py
│       ├── shared
│       │   ├── domain
│       │   │   ├── __init__.py
│       │   │   └── money.py
│       │   └── __init__.py
│       ├── __init__.py
│       └── py.typed
├── tests
│   └── test_order.py
├── .gitignore               you add the harness lines, commit
├── .harness.yaml            you add, commit
├── .importlinter            you add, commit
├── .python-version
├── README.md
├── compose.yaml             you add, commit
├── pyproject.toml
└── uv.lock
```

Git ignores the folders that the tree leaves out: `__pycache__/`, `.pytest_cache/` and `.mypy_cache/`.
The harness writes its run state, `.harness/state.db`, into the git directory, so the tree does not show it. `.gitignore` ignores `.harness/` in the project, except `.harness/plugins/`.
`harness gates init` writes no gates here, because you write them by hand.

## Gate paths

- The container gates have the same `paths`: `**/*.py`, `**/*.pyi`, `**/py.typed`, `tests/**`, `pyproject.toml`, `uv.lock`, `.python-version` and `compose.yaml`. The YAML anchor `python-inputs` shares the list.
- The setup step `py-install` has the `paths` `pyproject.toml`, `uv.lock`, `.python-version` and `compose.yaml`. These are the files that decide the content of the venv.
- mypy reads the `.pyi` stubs and the `py.typed` markers. `tests/**` holds the test data that is not Python. Add the folders of your own test data.
- Add each tool config that is not in `pyproject.toml`, such as `mypy.ini`, `pytest.ini`, `setup.cfg` or `tox.ini`.
- Commit `uv.lock`. It is in the setup paths and in the gate paths, so a dependency change runs the setup step and the gates again.
- `.python-version` is in the paths, because uv reads it to choose the Python.
- `compose.yaml` is in the paths. The image is not in the input hash, so an image change must change this file.
- The gate `lint` has no `paths`, so it hashes the full tree. A change to `pyproject.toml`, `.importlinter`, a pack or a plugin then runs it again.
- `mypy src tests` reads the folders of your own code. Add a folder to the command when your code lives elsewhere.
- On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
- A file that a gate reads but that its `paths` do not match does not run the gate again. Then `harness verify` counts a stale result in `cached`.
