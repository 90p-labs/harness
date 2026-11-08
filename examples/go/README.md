# Go example

This example sets up the harness on a Go module. Each gate runs in a Docker container. The harness and the host hooks run on the host.
It also shows the architecture rules of the `lint-setup` skill with go-arch-lint, and the canaries that prove them.

## Files

The example holds only the files that you add to your module:

- `compose.yaml`: one service `go`. It uses the image `golang:1.27.3-bookworm`, pinned with its index digest. It mounts the project at `/src`. It keeps the module cache and the build cache in two named volumes, so a second run is fast.
- `.harness.yaml`: the keys `plugins`, `lint`, `verify`, `setup` and `gates`.
  - `plugins` holds the `go-arch-lint` plugin. It answers `lint.check`. `lint.packs` names the custom pack.
  - `verify.parallel: 2` lets the harness run two gates at the same time.
  - `setup` holds one step, `go-mod`. It runs `go mod download` before the gates.
  - `gates` holds `go-fmt`, `go-vet`, `go-test`, `lint` and `lint-canaries`. `go-test` has `needs: [go-fmt, go-vet]`. `lint` and `lint-canaries` run on the host.
  - Each Go command runs `docker compose run --rm -T go ...`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.
- `.go-arch-lint.yml`: the go-arch-lint config. It has the components and the allowed imports. You own this file.
- `lint/architecture/pack.yaml`: the custom lint pack `architecture`. Its three rules name the engine `go-arch-lint`.
- `.harness/plugins/go-arch-lint.sh`: the lint engine plugin. It runs go-arch-lint and maps each violation to a rule id of the catalogue.
- `lint/rules.yaml`: the manifest. It gives a canary or a waiver for each rule of the `ddd` catalogue pack.
- `lint/canaries/`: one folder for each rule with a canary. Each folder has its own `go.mod`, so it is not part of your module. Each folder breaks its rule on purpose.

Set the top-level `name:` of `compose.yaml` to the name of your module. `harness verify --commits` runs the gates in a temporary worktree. The fixed name lets that worktree use the same cache volumes.

## Prerequisites

- Docker and Docker Compose. The output below comes from Docker Engine 29.9.2 and Compose v5.6.1.
- The `harness` binary, `sh`, `git` and `jq` on the `PATH` of the host. See the root README for the install steps. The go-arch-lint plugin runs on the host and uses `jq`. The proof used jq 1.6.
- A Go module with `go.mod` at the top of a git repository. Put the code in `internal/<context>/{domain,application,infrastructure}`. The plugin reads the context and the layer from these folder names. The canaries keep their own module name `example.com/shop`, so your module name does not matter.
- go-arch-lint v1.19.1. The plugin runs it in the container with `go run github.com/fe3dback/go-arch-lint@v1.19.1`, so you install nothing. The module cache volume keeps the download. The first run downloads and builds the tool. The proof took 15 seconds for this first run and 1 second for a later run.

You do not need Go on the host. The container has Go.

## Start the container

Pull the image before the first `harness verify`. A pull inside a gate counts against the gate timeout.

```text
$ time docker compose pull
 Image golang:1.27.3-bookworm@sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0 Pulling 
 Image golang:1.27.3-bookworm@sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0 Pulled 
docker compose pull  0.06s user 0.06s system 10% cpu 1.077 total
$ docker image ls golang
REPOSITORY   TAG       IMAGE ID       CREATED      SIZE
golang       <none>    dc0963f32053   3 days ago   1.25GB
```

The listing leaves out one row for an unrelated `golang` image on the test host.

A first pull downloads 290 MB of compressed layers on `linux/arm64` and 300 MB on `linux/amd64`, as the registry manifest gives them. The time depends on your network. The image is 1.25GB on disk. The pull above took 1.08 seconds, because the Docker store on the test host already held the layers.

## Step 1: see what the built-in detection writes

The harness detects a Go module from `go.mod`. On a bare module, `harness init --host claude` writes gates that run on the host:

```text
$ harness init --host claude
lint packs: none detected
gates: wrote go-fmt, go-vet, go-test
lint init: wrote .harness-lint
lint baseline: skipped, no lint pack
host: wrote /path/to/app/.claude/settings.json
$ cat .harness.yaml
gates:
  - id: go-fmt
    run: test -z "$(gofmt -l .)"
    paths:
      - '**/*.go'
  - id: go-vet
    run: go vet ./...
  - id: go-test
    run: go test ./...
```

These gates need Go on the host. The example keeps the same ids and runs each command in the container. Its `go-fmt` also fails on a file that `gofmt` cannot parse, and it prints the file names. `gofmt` also reads `testdata/`, so a module that keeps an invalid `.go` file there on purpose needs a narrower command.

## Step 2: copy the example and run the setup

Copy the files of the example to the top of your module and run the setup:

```text
$ cp -R /path/to/harness/examples/go/{compose.yaml,.harness.yaml,.go-arch-lint.yml,.harness,lint} .
$ harness init --host claude
lint packs: kept ./lint/architecture
next: run the lint-setup skill to set up the architecture rules for Go
gates: kept
lint init: wrote .harness-lint
lint baseline: wrote .harness-baseline/lint.json
host: wrote /path/to/app/.claude/settings.json
```

The setup keeps the `gates` key and the lint pack of the example. `harness lint init` writes `.harness-lint/VERSION`. `harness lint baseline` writes `.harness-baseline/lint.json` with no findings.
The `next:` line points to the `lint-setup` skill. The example already holds the files that the skill writes, so you can skip it.
`harness init` always installs the hooks. With the harness plugin, each hook then runs twice. See the root README for the setup with the plugin.

Add these lines to `.gitignore`. The first line ignores the run state of the harness. The second line keeps the plugin in git:

```text
.harness/*
!.harness/plugins/
```

## Step 3: run the gates two times

Commit the files before you run `harness verify`. The first run executes the setup step `go-mod`, and then each gate in the container.
The setup step prints `setup go-mod <duration>` before the gate lines. The second run changes no input. The setup step prints nothing, and each gate is `CACHED`:

```text
$ git add -A && git commit -q -m "chore: add the harness"
$ harness verify
setup go-mod 301ms
PASS go-fmt 442ms
PASS go-vet 506ms
PASS go-test 573ms
PASS lint 1.252s
PASS lint-canaries 2.876s
$ harness verify
CACHED go-fmt 442ms
CACHED go-vet 506ms
CACHED go-test 573ms
CACHED lint 1.251s
CACHED lint-canaries 2.876s
```

`verify.parallel: 2` lets the harness run two gates at the same time. `go-fmt` and `go-vet` have no `needs`, so they start together.
`go-test` has `needs: [go-fmt, go-vet]`. It starts only after both gates are `PASS` or `CACHED`.
`lint` and `lint-canaries` have no `needs`. The canary folders have their own `go.mod`, so `go vet ./...` and `go test ./...` skip them.

## A failed gate skips the gates that need it

Break the format of a file. This run uses `internal/order/application/place_order.go` with one line that has no indent:

```text
$ harness verify
FAIL go-fmt 476ms
PASS go-vet 537ms
SKIPPED go-test needs go-fmt
PASS lint 1.041s
PASS lint-canaries 2.653s

go-fmt:
 Container my-go-app-go-run-da0ac9e82843 Creating 
 Container my-go-app-go-run-da0ac9e82843 Created 
internal/order/application/place_order.go


error: gates failed
$ echo $?
1
```

`go-fmt` fails and prints the name of the file. `go-vet` does not need `go-fmt`, so it runs and passes. `go-test` needs `go-fmt`, so the harness skips it. `lint` runs again, because it hashes all the files, and it passes. `harness verify` exits with code 1.
The setup step does not run, because no file of its `paths` changed.

Restore the file. The inputs return to the state of the last pass, so each gate is `CACHED`:

```text
$ git checkout internal/order/application/place_order.go
Updated 1 path from the index
$ harness verify
CACHED go-fmt 418ms
CACHED go-vet 490ms
CACHED go-test 659ms
CACHED lint 1.042s
CACHED lint-canaries 2.818s
```

## A new dependency runs the setup step again

The setup step has the `paths` `go.mod`, `go.sum` and `compose.yaml`. A change to one of them runs the step again.
This run adds `go-cmp` to the module, so `go.mod` changes. `go.mod` is also in the `paths` of the gates, so the gates run again too:

```text
$ docker compose run --rm -T go go get github.com/google/go-cmp@v0.7.0
 Container my-go-app-go-run-ebeb6139ced8 Creating 
 Container my-go-app-go-run-ebeb6139ced8 Created 
go: added github.com/google/go-cmp v0.7.0
$ git diff --stat
 go.mod | 2 ++
 1 file changed, 2 insertions(+)
$ harness verify
setup go-mod 346ms
PASS go-fmt 467ms
PASS go-vet 521ms
PASS go-test 541ms
PASS lint 1.121s
PASS lint-canaries 2.559s
$ harness verify
CACHED go-fmt 466ms
CACHED go-vet 521ms
CACHED go-test 541ms
CACHED lint 1.121s
CACHED lint-canaries 2.559s
```

The next run prints no setup line. The step runs only when the hash of its `paths` changes.

`harness verify --commits` checks each commit in a temporary worktree. The gates run in the same container, with the same cache volumes. The setup step runs there only when the `go.mod`, `go.sum` or `compose.yaml` of the commit differ from its last pass:

```text
$ git add -A && git commit -q -m "chore: add go-cmp"
$ harness verify --commits HEAD~1
PASS d1d390622b0d chore
```

## Architecture rules with go-arch-lint

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
    waived: go-arch-lint checks the imports, not the file names.
```

The project does not turn on the built-in `ddd` pack, because its engine reads TypeScript.
The custom pack `architecture` gives the three rules with a canary to the engine `go-arch-lint`. Its scope `internal/**/*.go` leaves out `lint/canaries/`.
`.go-arch-lint.yml` has `workdir: internal`, so a canary never fails the `lint` gate.
The one waiver is `ddd/file-suffix`. go-arch-lint checks the imports of a package, not the names of the files.

`.go-arch-lint.yml` has one component for each layer of each context, for example `order-domain` for `internal/order/domain`. The `deps` key lets the domain use nothing, the application layer use the domain, and the infrastructure layer use the domain, the application layer and any third-party library.
The key `depOnAnyVendor: false` makes a third-party library a violation for the domain and the application layer. The standard library is always allowed. Test files are not checked.
The `in` globs end with `/**`, so a component also covers the use case folders below its layer folder. A component never lists a component of another context, so each context stays apart.
Edit the components and the `deps` to match your project. Then run `harness lint canaries` to see that each rule still catches its canary.

The plugin `go-arch-lint.sh` does these steps:

1. It reads the event from stdin before it starts the container.
2. It reads the folder of the call with `git rev-parse --show-prefix`. The harness runs the plugin in the canary folder for a canary, and in the project root for `harness lint`.
3. It runs `go-arch-lint check --json` in the container with `--project-path` set to that folder. The config is always `/src/.go-arch-lint.yml`. A canary folder has its own `go.mod` with the module path `example.com/shop`, and its imports use this path, so the tool resolves the imports. A canary folder holds every component folder, because the tool fails with `not found directories` for a missing folder. The folders without code hold an empty `.gitkeep`. No `.go` file sits directly in an `application/` folder: the real canaries are in `application/place_order/` and `application/issue_invoice/`.
4. A missing config gives no JSON, so the plugin prints the tool text to stderr and exits with 1. A config with a warning stops the plugin with an error that names the warning.
5. It gives the event and the report to `jq` on stdin. An import of another context is a `ddd/cross-context` finding. A violation of a `domain` component is a `ddd/domain-purity` finding. A violation of an `application` component is a `ddd/layer-direction` finding.
6. It keeps only the files and the rules of the event.

A domain file that imports a third-party library gives a finding. The proof added `internal/order/domain/identity.go` that imports `github.com/google/uuid`:

```console
$ harness lint
error ddd/domain-purity internal/order/domain/identity.go:3 order-domain must not depend on github.com/google/uuid
error: lint found error findings
$ echo $?
1
```

### The canaries pass

`harness lint canaries` sends `lint.check` to the engine of each rule with a canary, in the canary folder, with only that rule. The rule must be in the findings. No agent runs in this check:

```console
$ harness lint canaries
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
PASS ddd/layer-direction lint/canaries/ddd-layer-direction
PASS ddd/cross-context lint/canaries/ddd-cross-context
WAIVED ddd/file-suffix: go-arch-lint checks the imports, not the file names.
$ echo $?
0
```

### A config edit breaks a rule

The proof let the application layer of `order` use the infrastructure layer:

```diff
   order-application:
-    mayDependOn: [order-domain]
+    mayDependOn: [order-domain, order-infrastructure]
```

go-arch-lint then gave no finding for the canary `ddd-layer-direction`, and the check failed:

```console
$ harness lint canaries
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
FAIL ddd/layer-direction lint/canaries/ddd-layer-direction: the engine go-arch-lint gave no ddd/layer-direction finding
PASS ddd/cross-context lint/canaries/ddd-cross-context
WAIVED ddd/file-suffix: go-arch-lint checks the imports, not the file names.
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
WAIVED ddd/file-suffix: go-arch-lint checks the imports, not the file names.
$ echo $?
0
```

The output shows each waiver with its reason, so a review sees it. The proof then restored `.go-arch-lint.yml` and `lint/rules.yaml`.

## The project after the setup

The tree comes from `tree -a -I "vendor|node_modules|.git|.venv|var" --dirsfirst` in the project of the proof.
The marks on the right are not part of the `tree` output. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.

```text
.
├── .claude
│   └── settings.json                                                                           harness writes (host install), commit
├── .harness
│   └── plugins
│       └── go-arch-lint.sh                                                                     you add, commit
├── .harness-baseline
│   └── lint.json                                                                               harness writes (lint baseline), commit
├── .harness-lint
│   └── VERSION                                                                                 harness writes (lint init), commit
├── internal
│   ├── billing
│   │   ├── application
│   │   │   └── issue_invoice.go
│   │   ├── domain
│   │   │   └── invoice.go
│   │   └── infrastructure
│   │       └── invoice_table.go
│   └── order
│       ├── application
│       │   └── place_order.go
│       ├── domain
│       │   ├── order.go
│       │   └── order_test.go
│       └── infrastructure
│           └── order_table.go
├── lint
│   ├── architecture
│   │   └── pack.yaml                                                                           you add, commit
│   ├── canaries
│   │   ├── ddd-cross-context
│   │   │   ├── internal
│   │   │   │   ├── billing
│   │   │   │   │   ├── application
│   │   │   │   │   │   ├── issue_invoice
│   │   │   │   │   │   │   └── issue_invoice.go                                                you add, commit
│   │   │   │   │   │   └── .gitkeep                                                            you add, commit
│   │   │   │   │   ├── domain
│   │   │   │   │   │   └── doc.go                                                              you add, commit
│   │   │   │   │   └── infrastructure
│   │   │   │   │       └── doc.go                                                              you add, commit
│   │   │   │   └── order
│   │   │   │       ├── application
│   │   │   │       │   └── .gitkeep                                                            you add, commit
│   │   │   │       ├── domain
│   │   │   │       │   └── doc.go                                                              you add, commit
│   │   │   │       └── infrastructure
│   │   │   │           └── doc.go                                                              you add, commit
│   │   │   └── go.mod                                                                          you add, commit
│   │   ├── ddd-domain-purity
│   │   │   ├── internal
│   │   │   │   ├── billing
│   │   │   │   │   ├── application
│   │   │   │   │   │   └── .gitkeep                                                            you add, commit
│   │   │   │   │   ├── domain
│   │   │   │   │   │   └── doc.go                                                              you add, commit
│   │   │   │   │   └── infrastructure
│   │   │   │   │       └── doc.go                                                              you add, commit
│   │   │   │   └── order
│   │   │   │       ├── application
│   │   │   │       │   └── .gitkeep                                                            you add, commit
│   │   │   │       ├── domain
│   │   │   │       │   └── order.go                                                            you add, commit
│   │   │   │       └── infrastructure
│   │   │   │           └── doc.go                                                              you add, commit
│   │   │   └── go.mod                                                                          you add, commit
│   │   └── ddd-layer-direction
│   │       ├── internal
│   │       │   ├── billing
│   │       │   │   ├── application
│   │       │   │   │   └── .gitkeep                                                            you add, commit
│   │       │   │   ├── domain
│   │       │   │   │   └── doc.go                                                              you add, commit
│   │       │   │   └── infrastructure
│   │       │   │       └── doc.go                                                              you add, commit
│   │       │   └── order
│   │       │       ├── application
│   │       │       │   ├── place_order
│   │       │       │   │   └── place_order.go                                                  you add, commit
│   │       │       │   └── .gitkeep                                                            you add, commit
│   │       │       ├── domain
│   │       │       │   └── doc.go                                                              you add, commit
│   │       │       └── infrastructure
│   │       │           └── doc.go                                                              you add, commit
│   │       └── go.mod                                                                          you add, commit
│   └── rules.yaml                                                                              you add, commit
├── .gitignore                                                                                  you add the harness lines, commit
├── .go-arch-lint.yml                                                                           you add, commit
├── .harness.yaml                                                                               you add the plugin, the pack and the lint gates, commit
├── compose.yaml                                                                                you add, commit
├── go.mod
└── go.sum                                                                                      go writes, commit

49 directories, 42 files
```

The harness keeps the state of `harness verify` in `.git/.harness/state.db`. The `.harness/` folder of the tree holds only the plugin. The harness writes its run folders there when you run `harness run`. `.gitignore` ignores them, except `.harness/plugins/`.
The harness writes `.claude/settings.json`, `.harness-lint/VERSION` and `.harness-baseline/lint.json` when you run `harness init --host claude`.
`go.sum` appeared after the proof added the dependency `go-cmp`. A module with no dependency has no `go.sum`.

## The gate paths

The input hash of a gate covers the files that match its `paths`:

- `**/*.go`: the Go sources.
- `**/testdata/**`, `go.work` and `go.work.sum`: the other files that `go vet` and `go test` read. Add the files that a `//go:embed` line reads, if they are not in `testdata/`.
- `go.mod` and `go.sum`: the dependencies. Commit `go.sum`. A module with no dependency has no `go.sum`, and the path then matches nothing.
- `compose.yaml`: the image is not in the input hash, so a change of the image must change a file in the hash. Change the image in `compose.yaml` and the gates run again.

The setup step `go-mod` has the `paths` `go.mod`, `go.sum` and `compose.yaml`. Its hash does not cover the `.go` files.

A file that a gate reads but that its `paths` do not match does not run the gate again. Then `harness verify` prints `CACHED` for a stale result.

The harness leaves out the files that `.gitignore` lists. Do not ignore a file that a gate needs. Add `.harness/` to `.gitignore`: it holds the run folders.

## Where each part runs

- The host runs `harness`, the hooks, the guard and the plugin script `go-arch-lint.sh`.
- The container runs `gofmt`, `go vet`, `go test` and go-arch-lint.
- The named volumes `go-mod` and `go-build` keep the caches between runs.

On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
