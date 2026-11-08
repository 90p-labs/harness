# Python example

This example shows how a Python project with uv sets up the harness.
It shows two patterns that the other examples do not show:

- Gates that you write by hand in the `gates` key. The harness has no Python detection, and no plugin writes the gates.
- A custom lint pack with a lint engine plugin. The plugin runs ruff in the container and returns its diagnostics as findings.

The gates and ruff run in a Docker Compose service. The harness, the hooks and the plugin script run on the host.

## Files

The example holds only the files that you add to your project:

| File | Purpose |
|---|---|
| `compose.yaml` | The service `python`. It uses the pinned `uv` image and keeps the venv in a named volume. |
| `.harness.yaml` | The `ruff` plugin, the lint pack and the three gates. |
| `lint/python/pack.yaml` | The custom lint pack `python`. Its one rule names the engine `ruff`. |
| `.harness/plugins/ruff-lint.sh` | The lint engine plugin. It answers `lint.check`. |

## Prerequisites

1. Install Docker and Compose. The proof used Docker Engine 29.9.2 and Compose v5.6.1.
2. Put `harness`, `sh` and `jq` on the `PATH` of the host. The plugin script runs on the host and uses `jq`.
3. Use a uv project with `pytest`, `mypy` and `ruff` in the `dev` group: `uv add --dev pytest mypy ruff`. Then `uv.lock` pins them.

The proof used `pytest` 9.2.0, `mypy` 2.4.1 and `ruff` 0.17.4, from `uv init --lib`.
The image is `ghcr.io/astral-sh/uv:0.13.7-python3.14-trixie-slim`. It holds uv 0.13.7 and Python 3.14.10.
The `requires-python` of the project must accept Python 3.14.

The project needs no Node, no Biome, no oxlint and no `lint.disabled`.
The harness calls a lint engine only when an active pack has a rule for that engine.
The pack `python` has rules only for the engine `ruff`, so the harness never starts Biome or oxlint.
Each command below ran with a `PATH` that held only `harness`, `docker`, `docker-credential-desktop`, `jq`, `tree`, `/usr/bin` and `/bin`.

## What runs where

- The harness, the hooks, the gate `lint` and the plugin script run on the host.
- The gates `py-test` and `py-typecheck` and the ruff check run in the container with `docker compose run --rm -T python`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.
- `uv run --locked --exact` syncs the venv before it runs the tool. `--locked` stops when `uv.lock` does not match `pyproject.toml`. `--exact` removes each package that `uv.lock` does not hold. Without it, a package that a commit removed stays in the shared venv, and an import of it still passes. The temporary worktree of `harness verify --commits` uses the same venv volume, and the sync puts the packages of the checked commit into it.
- The venv is in the named volume `venv`, at `UV_PROJECT_ENVIRONMENT=/opt/venv`. The project gets no `.venv/`, and the host holds no Linux binary.
- The uv cache is in the named volume `uv-cache`. The cache and the venv are on two file systems, so uv cannot hardlink. Without `UV_LINK_MODE=copy`, uv prints "Failed to hardlink files; falling back to full copy" on each install.
- `UV_PYTHON_DOWNLOADS=never` makes uv use the Python of the image. A `requires-python` that the image does not match then stops the gate, and uv downloads no other Python. With `requires-python = ">=3.15"`, uv exits with code 2: "The Python request from `.python-version` resolved to Python 3.14.10, which is incompatible with the project's Python requirement".

Set the top-level `name:` of `compose.yaml` to the name of your project.
`harness verify --commits` runs the gates in a temporary worktree. The fixed name lets that worktree use the same venv volume.
A second checkout that runs gates at the same time must export `COMPOSE_PROJECT_NAME` with a different value. Two syncs into one venv can break each other.
For the same reason, do not run `harness lint` while `harness verify` runs in the same checkout. The plugin syncs the same venv.
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
$ mkdir -p .harness/plugins lint/python
$ cp /path/to/harness/examples/python/.harness/plugins/ruff-lint.sh .harness/plugins/
$ cp /path/to/harness/examples/python/lint/python/pack.yaml lint/python/
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
`.harness/*` ignores the run state of the harness, and `!.harness/plugins/` keeps the plugin in git.

The `gates` key of `.harness.yaml` holds three gates, written by hand:

- `py-test` runs `uv run --locked --exact pytest` in the container.
- `py-typecheck` runs `uv run --locked --exact mypy .` in the container. Put the mypy settings, such as `strict`, in the `[tool.mypy]` table of `pyproject.toml`.
- `lint` runs `harness lint --json` on the host, with the baseline `.harness-baseline/lint.json`.

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
With an empty venv volume, the first `harness lint` installed 13 packages, with 26 MiB of downloads, and took 2.0 s on the test host.
The next run took 0.7 s. The example sets `timeout: 60s`, so a slow network does not fail the plugin.

```console
$ harness lint init
$ cat .harness-lint/VERSION
aa057ac+1c0f1ffb7dd5
$ time harness lint
Using CPython 3.14.10 interpreter at: /usr/local/bin/python3.14
Creating virtual environment at: /opt/venv
   Building shop @ file:///app
      Built shop @ file:///app
Downloading pygments (1.2MiB)
Downloading ruff (9.7MiB)
Downloading ast-serialize (1.2MiB)
Downloading mypy (13.7MiB)
 Downloaded ast-serialize
 Downloaded pygments
 Downloaded ruff
 Downloaded mypy
Installed 13 packages in 129ms
error python/ruff src/shop/cart.py:1 F401 `os` imported but unused
error: lint found error findings

real	0m1.974s
user	0m0.098s
sys	0m0.060s
$ time harness lint
error python/ruff src/shop/cart.py:1 F401 `os` imported but unused
error: lint found error findings

real	0m0.664s
user	0m0.096s
sys	0m0.058s
```

`.harness-lint/VERSION` holds the binary version, a `+` and a hash of the custom pack files. Run `harness lint init` again after each edit of `lint/python/`.
The uv lines are the stderr of the plugin. The second run prints only the finding.

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
Commit `.harness.yaml`, `compose.yaml`, `.gitignore`, `.harness/plugins/`, `lint/`, `.harness-lint/`, `.harness-baseline/` and `.claude/`.

### 5. Verify two times

```console
$ harness verify
PASS py-test 1.294s
PASS py-typecheck 1.372s
PASS lint 426ms
$ harness verify
CACHED py-test 1.294s
CACHED py-typecheck 1.371s
CACHED lint 426ms
```

The first run prints `PASS`. The gate `lint` passes, because the baseline holds the finding.
The second run prints `CACHED`, because no input changed.

### 6. See the ratchet stop a new finding

The proof added `import json` and an empty line at the top of `tests/test_cart.py`:

```console
$ harness verify
PASS py-test 978ms
PASS py-typecheck 765ms
FAIL lint 397ms

lint:
{"version":1,"findings":[{"rule":"python/ruff","severity":"error","file":"src/shop/cart.py","line":1,"message":"F401 `os` imported but unused"},{"rule":"python/ruff","severity":"error","file":"tests/test_cart.py","line":1,"message":"F401 `json` imported but unused"}]}

error: gates failed
```

The gate `lint` fails on the finding in `tests/test_cart.py`, because the baseline does not hold it.
The report also lists the finding in the baseline. The baseline key is the rule, the file and the message, not the line.
The proof then removed the new import.

### 7. Fix the old finding and shrink the baseline

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
PASS f7cbbabb0171 chore
```

## The project after the setup

The tree comes from `tree -a -I ".git|__pycache__|.pytest_cache|.mypy_cache" --dirsfirst` in the project of the proof.
The marks on the right are not part of the `tree` output. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.

```text
.
├── .claude
│   └── settings.json        harness writes (host install), commit
├── .harness
│   └── plugins
│       └── ruff-lint.sh     you add, commit
├── .harness-baseline
│   └── lint.json            harness writes (lint baseline), commit
├── .harness-lint
│   └── VERSION              harness writes (lint init), commit
├── lint
│   └── python
│       └── pack.yaml        you add, commit
├── src
│   └── shop
│       ├── __init__.py
│       ├── cart.py
│       └── py.typed
├── tests
│   └── test_cart.py
├── .gitignore               you add the harness lines, commit
├── .harness.yaml            you add, commit
├── .python-version
├── README.md
├── compose.yaml             you add, commit
├── pyproject.toml
└── uv.lock

10 directories, 16 files
```

Git ignores the folders that the tree leaves out: `__pycache__/`, `.pytest_cache/` and `.mypy_cache/`.
The harness writes its run state into `.harness/` when a run starts. `.gitignore` ignores it, except `.harness/plugins/`.
`harness gates init` does not write the `gates` key here, because you write it by hand.

## Gate paths

- The container gates have the same `paths`: `**/*.py`, `**/*.pyi`, `**/py.typed`, `tests/**`, `pyproject.toml`, `uv.lock`, `.python-version` and `compose.yaml`.
- mypy reads the `.pyi` stubs and the `py.typed` markers. `tests/**` holds the test data that is not Python. Add the folders of your own test data.
- Add each tool config that is not in `pyproject.toml`, such as `mypy.ini`, `pytest.ini`, `setup.cfg` or `tox.ini`.
- Commit `uv.lock`. It is in the gate paths, so a dependency change runs the gates again.
- `.python-version` is in the paths, because uv reads it to choose the Python.
- `compose.yaml` is in the paths. The image is not in the input hash, so an image change must change this file.
- The gate `lint` has no `paths`, so it hashes the full tree. A change to `pyproject.toml`, the pack or the plugin then runs it again.
- On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
- A file that a gate reads but that its `paths` do not match does not run the gate again. Then `harness verify` prints `CACHED` for a stale result.
