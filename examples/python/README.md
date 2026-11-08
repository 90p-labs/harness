# Python example

This example shows how a Python project with uv sets up the harness.
It shows two patterns that the other examples do not show:

- Gates that you write by hand in the `gates` key. The harness has no Python detection, and no plugin writes the gates.
- A custom lint pack with a lint engine plugin. The plugin runs ruff in the container and returns its diagnostics as findings.

It uses a `setup` step and `needs`, as the Go example does.
It shows the architecture rules of the `lint-setup` skill with import-linter, and the canaries that prove them.
The gates, ruff and import-linter run in a Docker Compose service. The harness, the hooks and the plugin scripts run on the host.

## Files

The example holds only the files that you add to your project:

| File | Purpose |
|---|---|
| `compose.yaml` | The service `python`. It uses the pinned `uv` image and keeps the venv in a named volume. |
| `.harness.yaml` | The `ruff` and `import-linter` plugins, the two lint packs, `verify`, the `setup` step `py-install` and the four gates. |
| `lint/python/pack.yaml` | The custom lint pack `python`. Its one rule names the engine `ruff`. |
| `.harness/plugins/ruff-lint.sh` | The lint engine plugin of ruff. It answers `lint.check`. |
| `.importlinter` | The import-linter config: the contracts. You own this file. |
| `lint/architecture/pack.yaml` | The custom lint pack `architecture`. Its three rules name the engine `import-linter`. |
| `.harness/plugins/import-linter.sh` | The lint engine plugin of import-linter. It answers `lint.check`, runs `lint-imports` and maps each broken contract to a rule id of the catalogue. |
| `lint/rules.yaml` | The manifest. It gives a canary or a waiver for each rule of the `ddd` catalogue pack. |
| `lint/canaries/` | One folder for each rule with a canary. Each folder breaks its rule on purpose. |

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

- The harness, the hooks, the gates `lint` and `lint-canaries` and the plugin scripts run on the host.
- The setup step `py-install`, the gates `py-test` and `py-typecheck`, the ruff check and the import-linter check run in the container with `docker compose run --rm -T python`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.
- The setup step `py-install` runs `uv sync --locked` before the gates. `--locked` stops when `uv.lock` does not match `pyproject.toml`. `uv sync` also removes each package that `uv.lock` does not hold. Without that, a package that a commit removed stays in the shared venv, and an import of it still passes.
- The gates run `uv run --no-sync`. They use the venv as the setup step left it, and they do not sync a second time. The container gates `py-typecheck` and `py-test` do not write to the venv.
- The ruff plugin and the import-linter plugin run `uv run --locked --exact`. Each keeps its own sync, because `harness lint` also runs outside `harness verify`, with no setup step before it. `--exact` removes each package that `uv.lock` does not hold.
- The temporary worktree of `harness verify --commits` uses the same venv volume. The setup step runs there when the hash of its `paths` differs from the hash of its last pass, and the sync puts the packages of the checked commit into the venv.
- The venv is in the named volume `venv`, at `UV_PROJECT_ENVIRONMENT=/opt/venv`. The project gets no `.venv/`, and the host holds no Linux binary.
- The uv cache is in the named volume `uv-cache`. The cache and the venv are on two file systems, so uv cannot hardlink. Without `UV_LINK_MODE=copy`, uv prints "Failed to hardlink files; falling back to full copy" on each install.
- `UV_PYTHON_DOWNLOADS=never` makes uv use the Python of the image. A `requires-python` that the image does not match then stops the gate, and uv downloads no other Python. With `requires-python = ">=3.15"`, uv exits with code 2: "The Python request from `.python-version` resolved to Python 3.14.10, which is incompatible with the project's Python requirement".

Set the top-level `name:` of `compose.yaml` to the name of your project.
`harness verify --commits` runs the gates in a temporary worktree. The fixed name lets that worktree use the same venv volume.
A second checkout that runs gates at the same time must export `COMPOSE_PROJECT_NAME` with a different value. Two syncs into one venv can break each other.
The harness keeps one setup pass for the repo, so it can skip the setup step in that checkout, and its new venv volume stays empty.
Run `docker compose run --rm -T python uv sync --locked` once in that checkout.
For the same reason, do not run `harness lint` or `harness lint canaries` while `harness verify` runs in the same checkout. The plugins sync the same venv.
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

The proof started from a `uv init --lib` project with `src/shop/cart.py`, `tests/test_cart.py`, `uv.lock`, `.gitignore` and the `compose.yaml` of this example, in one commit. The proof ran `uv init` and `uv add` in the container.
The project also had two bounded contexts: `src/shop/order` with the folders `domain`, `application` and `infrastructure`, and `src/shop/billing` with the folder `domain`. Each folder is a package.
`src/shop/cart.py` had one unused import, `import os`.
The paths in the output below show `/path/to/app` for the project folder.

### 1. See that no detection runs

```console
$ harness gates init
$ cat .harness.yaml
gates: []
```

`harness gates init` finds no toolchain that it knows, so it prints nothing and writes an empty `gates` key.
Remove this `.harness.yaml` before the next step.

### 2. Copy the example

```console
$ cp /path/to/harness/examples/python/compose.yaml /path/to/harness/examples/python/.harness.yaml .
$ cp /path/to/harness/examples/python/.importlinter .
$ mkdir -p .harness/plugins lint/python lint/architecture
$ cp /path/to/harness/examples/python/.harness/plugins/ruff-lint.sh .harness/plugins/
$ cp /path/to/harness/examples/python/.harness/plugins/import-linter.sh .harness/plugins/
$ cp /path/to/harness/examples/python/lint/python/pack.yaml lint/python/
$ cp /path/to/harness/examples/python/lint/architecture/pack.yaml lint/architecture/
$ cp /path/to/harness/examples/python/lint/rules.yaml lint/
$ cp -R /path/to/harness/examples/python/lint/canaries lint/
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

The `setup` key of `.harness.yaml` holds one step, and the `gates` key holds four gates. You write them by hand:

- `py-install` is the setup step. It runs `uv sync --locked` in the container.
- `py-typecheck` runs `uv run --no-sync mypy src tests` in the container. Put the mypy settings, such as `strict`, in the `[tool.mypy]` table of `pyproject.toml`.
- `lint` runs `harness lint --json` on the host, with the baseline `.harness-baseline/lint.json`.
- `lint-canaries` runs `harness lint canaries` on the host. The `lint-setup` skill adds it by hand to the `gates` key.
- `py-test` runs `uv run --no-sync pytest` in the container. It has `needs: [py-typecheck, lint, lint-canaries]`.

`mypy` checks `src` and `tests`, not `.`. Each canary folder holds a package `shop`, so `mypy .` stops with "Duplicate module named shop".

`verify.parallel: 2` lets the harness run two gates at the same time.
`py-typecheck`, `lint` and `lint-canaries` need nothing, so two of them start together.
`py-test` starts only after `py-typecheck`, `lint` and `lint-canaries` are `PASS` or `CACHED`.
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
With an empty venv volume, the first `harness lint` took 2.9 s on the test host. The next run took 1.0 s.
The harness runs the engines one after the other, in the order of their names, so the import-linter plugin runs first.
That plugin keeps the uv text with the tool text, so the first run prints no uv line.
The example sets `timeout: 60s`, so a slow network does not fail the plugin.

```console
$ harness lint init
$ cat .harness-lint/VERSION
dev+2fc264a55939
$ time harness lint
error python/ruff src/shop/cart.py:1 F401 `os` imported but unused
error: lint found error findings

real	0m2.942s
user	0m0.155s
sys	0m0.087s
$ time harness lint
error python/ruff src/shop/cart.py:1 F401 `os` imported but unused
error: lint found error findings

real	0m0.994s
user	0m0.162s
sys	0m0.089s
```

`.harness-lint/VERSION` holds the binary version, a `+` and a hash of the custom pack files. Run `harness lint init` again after each edit of `lint/python/` or `lint/architecture/`.
The finding of the ruff pack is the only one, because the code has no broken contract.

### 4. Record the baseline and install the hooks

```console
$ harness lint baseline
$ cat .harness-baseline/lint.json
{
  "version": 1,
  "findings": [
    {
      "rule": "python/ruff",
      "severity": "error",
      "file": "src/shop/cart.py",
      "line": 1,
      "message": "F401 `os` imported but unused"
    }
  ]
}
$ harness host install --host claude
/path/to/app/.claude/settings.json
```

`harness lint` still exits with code 1 on this finding. Only the gate `lint` reads the baseline.
Commit `.harness.yaml`, `compose.yaml`, `.importlinter`, `.gitignore`, `.harness/plugins/`, `lint/`, `.harness-lint/`, `.harness-baseline/` and `.claude/`.

### 5. Verify two times

```console
$ time harness verify
setup py-install 273ms
PASS py-typecheck 749ms
PASS lint 1.099s
PASS lint-canaries 2.021s
PASS py-test 1.12s

real	0m4.268s
user	0m0.611s
sys	0m0.382s
$ time harness verify
CACHED py-typecheck 749ms
CACHED lint 1.098s
CACHED lint-canaries 2.021s
CACHED py-test 1.119s

real	0m0.098s
user	0m0.041s
sys	0m0.064s
```

The first run starts with `setup py-install`, because the step never passed before. Then `py-typecheck`, `lint` and `lint-canaries` run, two at a time, and `py-test` runs after the three.
The four gates take 5.0 s together (0.749 s, 1.099 s, 2.021 s and 1.12 s). The whole run took 4.3 s with the setup step, because the first three gates ran two at a time.
The gate `lint` passes, because the baseline holds the finding. The gate `lint-canaries` passes, because each canary breaks its rule.
The second run prints no `setup` line and prints `CACHED`, because no input changed. It took 0.1 s.

### 6. See the ratchet stop a new finding

The proof added `import json` and an empty line at the top of `tests/test_cart.py`:

```console
$ harness verify
CACHED py-typecheck 956ms
FAIL lint 1.235s
PASS lint-canaries 2.207s
SKIPPED py-test needs lint

lint:
{"version":1,"findings":[{"rule":"python/ruff","severity":"error","file":"src/shop/cart.py","line":1,"message":"F401 `os` imported but unused"},{"rule":"python/ruff","severity":"error","file":"tests/test_cart.py","line":1,"message":"F401 `json` imported but unused"}]}

error: gates failed
$ echo $?
1
```

The gate `lint` fails on the finding in `tests/test_cart.py`, because the baseline does not hold it.
`py-typecheck` prints `CACHED`, because the proof ran the same change once before. The gate `lint-canaries` passes, because the change does not touch a canary.
`py-test` needs `lint`, so the harness does not run it. It prints `SKIPPED py-test needs lint`, and `harness verify` exits with code 1.
The report also lists the finding in the baseline. The baseline key is the rule, the file and the message, not the line.
The proof then removed the new import. The next `harness verify` printed `CACHED` for the four gates, because the files were the same as in step 5.

### 7. Change a dependency

The setup step runs again when `uv.lock` changes. The proof added a dev dependency in the container, with `--no-sync` so that only `uv.lock` and `pyproject.toml` change:

```console
$ docker compose run --rm -T python uv add --no-sync --dev pytest-timeout
Resolved 21 packages in 527ms
$ git diff --stat
 pyproject.toml |  1 +
 uv.lock        | 14 ++++++++++++++
 2 files changed, 15 insertions(+)
$ time harness verify
setup py-install 435ms
PASS py-typecheck 756ms
PASS lint 1.125s
PASS lint-canaries 2.031s
PASS py-test 927ms

real	0m4.250s
user	0m0.597s
sys	0m0.362s
$ docker compose run --rm -T python uv pip list --python /opt/venv/bin/python | grep timeout
pytest-timeout    2.4.0
$ time harness verify
CACHED py-typecheck 756ms
CACHED lint 1.124s
CACHED lint-canaries 2.031s
CACHED py-test 927ms

real	0m0.096s
user	0m0.045s
sys	0m0.068s
```

The first run prints `setup py-install` and installs `pytest-timeout` into the venv. The second run, with no change, prints no `setup` line.
The proof then removed the dependency. The files returned to the state before the new dependency:

```console
$ docker compose run --rm -T python uv remove --no-sync --dev pytest-timeout
Resolved 20 packages in 12ms
$ time harness verify
setup py-install 288ms
CACHED py-typecheck 749ms
CACHED lint 1.15s
CACHED lint-canaries 2.143s
CACHED py-test 1.119s

real	0m0.389s
user	0m0.093s
sys	0m0.091s
$ docker compose run --rm -T python uv pip list --python /opt/venv/bin/python | grep -c timeout
0
```

The setup step ran again, because the hash of `uv.lock` changed. It removed `pytest-timeout` from the venv, because `uv sync` removes each package that `uv.lock` does not hold.
The gates are `CACHED`, because their input hash is the same as before the new dependency.

### 8. Fix the old finding and shrink the baseline

The proof removed `import os` from `src/shop/cart.py`:

```console
$ harness lint
$ harness lint baseline
$ cat .harness-baseline/lint.json
{
  "version": 1,
  "findings": []
}
```

`harness lint baseline` only removes the findings that no longer occur, so the baseline only shrinks.
Commit the fix and the baseline. `harness verify --commits` checks the commit in a temporary worktree:

```console
$ harness verify --commits HEAD~1
PASS 37030ac5fd91 chore
```

The setup step prints no line here, because the hash of its `paths` is the same as the hash of its last pass.

## Architecture rules with import-linter

The `lint-setup` skill wrote these files with its steps. `harness lint catalogue` prints the rules of the catalogue.
The manifest `lint/rules.yaml` names the catalogue pack `ddd` and gives each of its four rules a canary or a waiver:

```yaml
pack: ddd
rules:
  - id: ddd/domain-purity
    canary: lint/canaries/ddd-domain-purity
  - id: ddd/layer-direction
    canary: lint/canaries/ddd-layer-direction
  - id: ddd/cross-context
    canary: lint/canaries/ddd-cross-context
  - id: ddd/file-suffix
    waived: import-linter checks the imports, not the file names.
```

The project does not turn on the built-in `ddd` pack, because its engine reads TypeScript.
The custom pack `architecture` gives the three rules with a canary to the engine `import-linter`. Its scope `src/**/*.py` leaves out `lint/canaries/`.
`.importlinter` sets `root_package = shop`, so import-linter reads only `src/shop`, and a canary never fails the `lint` gate.
The ruff pack has the scope `**/*.py`. The canaries use each import that they write, so ruff reports nothing in them.

`.importlinter` has one contract for each rule. The `name` of a contract is the rule id, so the plugin needs no table:

- `ddd/domain-purity` is a `forbidden` contract. The source `shop.*.domain` must not import `shop.*.application`, `shop.*.infrastructure` or a framework. The example lists `sqlalchemy`, `django`, `flask` and `fastapi`. `include_external_packages = True` turns on the check of the packages that are not in `shop`. The package does not need to be installed.
- `ddd/layer-direction` is a `layers` contract. The containers `shop.*` each hold the layers `(infrastructure)` and `(application)`. The brackets make a layer optional, because `shop.billing` has only a domain.
- `ddd/cross-context` is an `independence` contract on the modules `shop.*`. The line `shop.** -> shop.*` lets a context import the package of another context, which is its public entry point, but not an inner module. `unmatched_ignore_imports_alerting = none` stops an error, because the project has no such import. The module `shop.cart` also counts as a context.

The layers contract does not hold `domain`. The domain contract already reports a domain import of the application layer or the infrastructure layer, so one broken import gives one rule id.
Edit the contracts to match your project. Then run `harness lint canaries` to see that each rule still catches its canary.

The plugin `import-linter.sh` does these steps:

1. It reads the event from stdin before it starts the container.
2. It reads the folder of the call with `git rev-parse --show-prefix`. The harness runs the plugin in the canary folder for a canary, and in the project root for `harness lint`.
3. It runs `lint-imports` in the container with `PYTHONPATH` set to `/app/<folder>src`. The package `shop` of the canary then comes before the package `shop` of the project. The config is `/app/.importlinter` for each call.
4. It sets `COLUMNS=1000`. Without a terminal, the report wraps at 80 columns and splits an import line in two.
5. It looks for the summary line `Contracts: <n> kept, <m> broken.` Without it, the config is not valid and there is no report. The plugin prints the tool text to stderr and exits with 1. A broken contract is a finding, not a plugin failure.
6. It reads the first import of each chain of the report with `awk`. The contract name is the rule id. `lint-imports` reports modules, with the line of the import.
7. It gives the event and the imports to `jq` on stdin. `jq` maps a module to `src/<path>.py` or `src/<path>/__init__.py`, and keeps the file that is in the payload. It keeps only the rules of the event, and one finding for each rule, file and line.

A report of a chain that goes through another module gives the file and the line of the first import of the chain.

A domain class that uses a framework gives a finding. The proof added `src/shop/order/domain/order_row.py` with `import sqlalchemy`:

```console
$ harness lint
error python/ruff src/shop/cart.py:1 F401 `os` imported but unused
error ddd/domain-purity src/shop/order/domain/order_row.py:1 shop.order.domain is not allowed to import sqlalchemy: shop.order.domain.order_row -> sqlalchemy
error: lint found error findings
$ echo $?
1
```

The ruff finding still shows. The proof then removed the file.

### The canaries pass

`harness lint canaries` sends `lint.check` to the engine of each rule with a canary, in the canary folder, with only that rule. The rule must be in the findings. No agent runs in this check:

```console
$ harness lint canaries
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
PASS ddd/layer-direction lint/canaries/ddd-layer-direction
PASS ddd/cross-context lint/canaries/ddd-cross-context
WAIVED ddd/file-suffix: import-linter checks the imports, not the file names.
$ echo $?
0
```

Each canary is a small `src/shop` package that mirrors the project:

- `ddd-domain-purity` has `shop.order.domain.order`, which imports `sqlalchemy`.
- `ddd-layer-direction` has `shop.order.application.place_order`, which imports `shop.order.infrastructure.order_table`.
- `ddd-cross-context` has `shop.order.application.place_order`, which imports the inner module `shop.billing.domain.invoice`.

### A config edit breaks a rule

The proof let the application layer use the infrastructure layer:

```diff
 layers =
     (infrastructure)
     (application)
+unmatched_ignore_imports_alerting = none
+ignore_imports =
+    shop.*.application.* -> shop.*.infrastructure.*
```

import-linter then gave no finding for the canary `ddd-layer-direction`, and the check failed:

```console
$ harness lint canaries
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
FAIL ddd/layer-direction lint/canaries/ddd-layer-direction: the engine import-linter gave no ddd/layer-direction finding
PASS ddd/cross-context lint/canaries/ddd-cross-context
WAIVED ddd/file-suffix: import-linter checks the imports, not the file names.
error: a lint canary failed
$ echo $?
1
```

The `lint` gate stays green after this edit, because the code has no violation. The `lint-canaries` gate fails, so `harness verify` fails.

### A waiver accepts the change

When the team wants the edit, it waives the rule in `lint/rules.yaml` with a reason:

```diff
   - id: ddd/layer-direction
-    canary: lint/canaries/ddd-layer-direction
+    waived: the team lets the application layer use the infrastructure layer.
```

```console
$ harness lint canaries
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
WAIVED ddd/layer-direction: the team lets the application layer use the infrastructure layer.
PASS ddd/cross-context lint/canaries/ddd-cross-context
WAIVED ddd/file-suffix: import-linter checks the imports, not the file names.
$ echo $?
0
```

The output shows each waiver with its reason, so a review sees it. The proof then restored `.importlinter` and `lint/rules.yaml`.

## The project after the setup

The tree comes from `tree -a -I "vendor|node_modules|.git|.venv|var|__pycache__|.pytest_cache|.mypy_cache" --dirsfirst` in the project of the proof.
The marks on the right are not part of the `tree` output. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.

```text
.
├── .claude
│   └── settings.json        harness writes (host install), commit
├── .harness
│   └── plugins
│       ├── import-linter.sh     you add, commit
│       └── ruff-lint.sh         you add, commit
├── .harness-baseline
│   └── lint.json            harness writes (lint baseline), commit
├── .harness-lint
│   └── VERSION              harness writes (lint init), commit
├── lint
│   ├── architecture
│   │   └── pack.yaml        you add, commit
│   ├── canaries
│   │   ├── ddd-cross-context
│   │   │   └── src
│   │   │       └── shop
│   │   │           ├── billing
│   │   │           │   ├── domain
│   │   │           │   │   ├── __init__.py
│   │   │           │   │   └── invoice.py        you add, commit
│   │   │           │   └── __init__.py
│   │   │           ├── order
│   │   │           │   ├── application
│   │   │           │   │   ├── __init__.py
│   │   │           │   │   └── place_order.py    you add, commit
│   │   │           │   └── __init__.py
│   │   │           └── __init__.py
│   │   ├── ddd-domain-purity
│   │   │   └── src
│   │   │       └── shop
│   │   │           ├── order
│   │   │           │   ├── domain
│   │   │           │   │   ├── __init__.py
│   │   │           │   │   └── order.py          you add, commit
│   │   │           │   └── __init__.py
│   │   │           └── __init__.py
│   │   └── ddd-layer-direction
│   │       └── src
│   │           └── shop
│   │               ├── order
│   │               │   ├── application
│   │               │   │   ├── __init__.py
│   │               │   │   └── place_order.py    you add, commit
│   │               │   ├── infrastructure
│   │               │   │   ├── __init__.py
│   │               │   │   └── order_table.py    you add, commit
│   │               │   └── __init__.py
│   │               └── __init__.py
│   ├── python
│   │   └── pack.yaml        you add, commit
│   └── rules.yaml           you add, commit
├── src
│   └── shop
│       ├── billing
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
│       │   │   └── order_table.py
│       │   └── __init__.py
│       ├── __init__.py
│       ├── cart.py
│       └── py.typed
├── tests
│   └── test_cart.py
├── .gitignore               you add the harness lines, commit
├── .harness.yaml            you add, commit
├── .importlinter            you add, commit
├── .python-version
├── README.md
├── compose.yaml             you add, commit
├── pyproject.toml
└── uv.lock

36 directories, 47 files
```

The `__init__.py` files of a canary are empty. You add and commit them with the canary.
Git ignores the folders that the tree leaves out: `__pycache__/`, `.pytest_cache/` and `.mypy_cache/`.
The harness writes its run state, `.harness/state.db`, into the git directory, so the tree does not show it. `.gitignore` ignores `.harness/` in the project, except `.harness/plugins/`.
`harness gates init` does not write the `gates` key here, because you write it by hand.

## Gate paths

- The container gates have the same `paths`: `**/*.py`, `**/*.pyi`, `**/py.typed`, `tests/**`, `pyproject.toml`, `uv.lock`, `.python-version` and `compose.yaml`. The YAML anchor `python-inputs` shares the list.
- The setup step `py-install` has the `paths` `pyproject.toml`, `uv.lock`, `.python-version` and `compose.yaml`. These are the files that decide the content of the venv.
- mypy reads the `.pyi` stubs and the `py.typed` markers. `tests/**` holds the test data that is not Python. Add the folders of your own test data.
- Add each tool config that is not in `pyproject.toml`, such as `mypy.ini`, `pytest.ini`, `setup.cfg` or `tox.ini`.
- Commit `uv.lock`. It is in the setup paths and in the gate paths, so a dependency change runs the setup step and the gates again.
- `.python-version` is in the paths, because uv reads it to choose the Python.
- `compose.yaml` is in the paths. The image is not in the input hash, so an image change must change this file.
- The gates `lint` and `lint-canaries` have no `paths`, so they hash the full tree. A change to `pyproject.toml`, `.importlinter`, a pack, a plugin or a canary then runs them again.
- `mypy src tests` reads the folders of your own code. Add a folder to the command when your code lives elsewhere.
- On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
- A file that a gate reads but that its `paths` do not match does not run the gate again. Then `harness verify` prints `CACHED` for a stale result.
