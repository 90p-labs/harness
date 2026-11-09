# Install and set up a repository

This page holds the binary details, the lint tools and the full setup of a repository, and the [README](../README.md) gives the short form.

## The plugin binary

The plugin starts the binary through `bin/harness`, which runs `dist/harness-OS-ARCH` of the plugin folder. When that binary is missing:

- A command downloads the binary of release `v<plugin version>` with `gh`. Without `gh`, or when the download fails, it builds the binary with Go. Without both, it exits 127 and names the fix.
- A hook never builds, because it must not block the session. It starts the `gh` download in the background and skips itself. A skipped hook prints `{}`, so the guard is off until the binary is there. When the download fails, the next hooks print its error, and a hook tries again after 10 minutes.
- On a machine with Go and no `gh`, start `/harness:harness status` once (Codex: `$harness:harness status`). The skill runs `bin/harness version`, which builds the binary.

The plugin puts no `harness` command on the `PATH`. The setup commands below and a gate such as `harness lint --json` need one: build it from source, or download the asset `harness-OS-ARCH` of release `v<plugin version>` (`gh release download v<plugin version> -R 90p-labs/harness -p harness-OS-ARCH`), make it executable and put it on the `PATH` as `harness`.

## Build from source

Use this path for a repository without the plugin, or to put `harness` on the `PATH`. The harness needs Go 1.27 (see `go.mod`):

```sh
git clone https://github.com/90p-labs/harness.git
cd harness
go install ./cmd/harness   # into $(go env GOPATH)/bin
make build                 # or: go build -o dist/harness ./cmd/harness
make check                 # gofmt, go vet and go test
./dist/harness version
make install-claude        # install this checkout as the Claude Code plugin, and again after each change
```

Without the plugin, `harness init` below writes the hooks into the repository.

## Lint tools

The lint packs call external tools. Put them on the `PATH`:

| Tool | Version | Needed for |
|---|---|---|
| Biome | 2.0 or later (CI pins 2.6.6) | the packs `ddd`, `ddd-es`, `nextjs` and `shadcn` |
| oxlint | CI pins 1.92.0 | the two oxlint rules of the pack `nextjs` |
| helm, promtool | any | the chart rules of the pack `observability`, and the `helm-render` and `promtool-test` gates of the default deploy plugins |

```sh
npm i -g @biomejs/biome@2.6.6 oxlint@1.92.0
```

The config is `.harness.yaml` at the git top level. Run the commands below from the repository root.

## With harness init (no plugin)

`harness init` always installs the hooks of the host. With the plugin, each hook then runs twice: use the step by step setup below.

`harness init --host claude` (or `--host codex`) sets up the repository in one step. It keeps `lint.packs`, or it chooses the packs from the repository files: `next` or `@nestjs/core` in `package.json` give `nextjs` or `ddd`, and `components.json` gives `shadcn`. Then it writes the gates, the lint files and the lint baseline, and installs the hooks. On a NestJS repository with the scripts `typecheck` and `test`:

```console
$ harness init --host claude
lint packs: wrote ddd
gates: wrote node-typecheck, node-test, lint
lint init: wrote .harness-lint
lint baseline: wrote .harness-lint/baseline.json (3 findings, run harness lint to see them)
host: wrote /home/dev/shop/.claude/settings.json
```

The baseline keeps the findings that exist today, so the `lint` gate fails only on new ones. Run `harness lint` to see them. Commit the host config .claude/settings.json (or `.codex/hooks.json`) with `.harness.yaml` and `.harness-lint/`, so each clone gets the hooks.

A second run keeps `lint.packs` and the `gates` key, and the other steps are safe to repeat. With no pack, it skips the baseline. With no gate to write, it prints `gates: none detected` and writes no `gates` key. An empty `gates` or `setup` list counts as absent. When it chooses packs but the `gates` key holds a gate, it stops before it writes: set `lint.packs` and the `lint` gate by hand, or remove the `gates` key. Without a lint engine on the `PATH`, it stops at the baseline step: install the engine and run it again. After a full run, do step 5 below.

## Step by step

1. Set the lint packs first, if you use them, for example `lint: {packs: [ddd]}`. `gates init` adds the lint gate only when `lint.packs` is set.
2. Run `harness lint init` to write the engine files of the packs into `.harness-lint/`. Lint init keeps `baseline.json`. Run `harness lint` to see the findings, and `harness lint baseline` to record them as the baseline. The lint gate fails only on a new `error` finding. A later `harness lint baseline` only removes the keys that no longer match, so the baseline only shrinks.
3. Run `harness gates init` to detect the gates. It reads the toolchain (Go, pnpm, npm, Biome) and the deploy plugins, and writes the `gates` key. It refuses to change an existing key.
4. Skip this step when you use the plugin: the plugin installs the hooks. Else run `harness host install --host claude` (or `--host codex`). It merges the hooks into `.claude/settings.json` or `.codex/hooks.json` and keeps every other key. `harness host capabilities --host codex` prints each host event and whether the host supports it.
5. Commit `.harness.yaml`, `.harness-lint/` and the host config, if you wrote one. Add `.harness/` to `.gitignore`: it holds the run folders.

## Example config

An example `.harness.yaml`. `harness config` prints the effective config, and an unknown key is an error:

```yaml
git:
  default_branch: main
run:
  approval: true          # wait for a human approval of each plan
  max_review_rounds: 3    # the last allowed rework blocks the run
roles:
  lead:
    claude: [opus]  # guidance for the session model, not a guard
  explorer:
    claude: [haiku]
  worker:
    claude: [sonnet, haiku]
  reviewer:
    claude: [sonnet, haiku, opus]  # the guard denies an agent call on another model
  advisor:
    claude: [fable]
lint:
  packs: [ddd]
verify:
  parallel: 3             # at most 3 gates at the same time; the default 1 runs them in file order
setup:
  - id: go-mod
    run: go mod download
    paths: [go.mod, go.sum]  # runs again only when these files change
gates:
  - id: go-fmt
    run: test -z "$(gofmt -l .)"
    paths: ["**/*.go"]
  - id: go-vet
    run: go vet ./...
  - id: lint
    run: harness lint --json
    baseline: .harness-lint/baseline.json
  - id: go-test
    run: go test ./...
    paths: ["**/*.go", go.mod, go.sum]   # a docs change does not run the gate again
    needs: [go-fmt, go-vet, lint]        # starts only when each need is PASS or CACHED
  - id: e2e
    run: make e2e
    needs: [go-test]
```

`harness verify` runs `go-mod` before the gates, and only when `go.mod` or `go.sum` changed since its last pass. A setup step has no evidence and no `needs`, and a failed step stops `harness verify` before any gate. `go-fmt`, `go-vet` and `lint` run in parallel. When `lint` fails, `go-test` and `e2e` do not run, and `harness verify` exits 1:

```text
PASS go-fmt 61ms
PASS go-vet 1.4s
FAIL lint 812ms
SKIPPED go-test needs lint
SKIPPED e2e needs go-test

lint:
<the last 4 KiB of the lint output>
```

[`docs/configuration.md`](configuration.md) explains every config key, its default and its env variable, with recipes. The [full configuration](configuration-full.md) page shows every key and one example for each case.
