# Go example

This example sets up the harness on a Go module. Each gate runs in a Docker container. The harness and the host hooks run on the host.
It also shows the architecture rules of the `lint-setup` skill with go-arch-lint.

## Files

The example holds only the files that you add to your module:

- `compose.yaml`: one service `go`. It uses the image `golang:1.27.3-bookworm`, pinned with its index digest. It mounts the project at `/src`. It keeps the module cache and the build cache in two named volumes, so a second run is fast.
- `.harness.yaml`: the keys `plugins`, `lint`, `verify`, `setup` and `gates`.
  - `plugins` holds the `go-arch-lint` plugin. It answers `lint.check`. `lint.packs` names the custom pack.
  - `verify.parallel: 2` lets the harness run two gates at the same time.
  - `setup` holds one step, `go-mod`. It runs `go mod download` before the gates.
  - `gates` holds `go-fmt`, `go-vet`, `go-test` and `lint`. `go-test` has `needs: [go-fmt, go-vet]`. `lint` runs on the host.
  - Each Go command runs `docker compose run --rm -T go ...`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.
- `.go-arch-lint.yml`: the go-arch-lint config. It has the components and the allowed imports. You own this file.
- `lint/architecture/pack.yaml`: the custom lint pack `architecture`. Its four rules name the engine `go-arch-lint`.
- `.harness/plugins/go-arch-lint.sh`: the lint engine plugin. It runs go-arch-lint and maps each violation to a rule id of the catalogue.

Set the top-level `name:` of `compose.yaml` to the name of your module. `harness verify --commits` runs the gates in a temporary worktree. The fixed name lets that worktree use the same cache volumes.

## Prerequisites

- Docker and Docker Compose. The output below comes from Docker Engine 29.9.2 and Compose v5.6.1.
- The `harness` binary, `sh`, `git` and `jq` on the `PATH` of the host. See the root README for the install steps. The go-arch-lint plugin runs on the host and uses `jq`. The proof used jq 1.6.
- A Go module with `go.mod` at the top of a git repository. Put the code in `internal/<context>/{domain,application,infrastructure}`. The plugin reads the context and the layer from these folder names.
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
next: run the lint-setup skill to set up the architecture rules for Go
gates: wrote go-fmt, go-vet, go-test
lint init: wrote .harness-lint
lint baseline: skipped, no lint pack
host: wrote /path/to/app/.claude/settings.json
gitignore: added .harness/runs/
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

The `next:` line points to the `lint-setup` skill, because no lint pack checks the Go code yet. The example already holds the files that the skill keeps, so you can skip it.
These gates need Go on the host. The example keeps the same ids and runs each command in the container. Its `go-fmt` also fails on a file that `gofmt` cannot parse, and it prints the file names. `gofmt` also reads `testdata/`, so a module that keeps an invalid `.go` file there on purpose needs a narrower command.

## Step 2: copy the example and run the setup

Copy the files of the example to the top of your module and run the setup:

```text
$ cp -R /path/to/harness/examples/go/{compose.yaml,.harness.yaml,.go-arch-lint.yml,.harness,lint} .
$ harness init --host claude
lint packs: kept ./lint/architecture
gates: kept
lint init: wrote .harness-lint
lint baseline: wrote .harness-lint/baseline.json (0 findings, run harness lint to see them)
host: wrote /path/to/app/.claude/settings.json
gitignore: kept
```

The setup keeps the `gates` key and the lint pack of the example. `harness lint init` writes `.harness-lint/VERSION`. `harness lint baseline` writes `.harness-lint/baseline.json` with no findings.
The `next:` line is gone, because the pack `architecture` now gives the rules with the `.go` scope to go-arch-lint.
`harness init` installs the hooks. With the `90p` plugin on Claude Code, it skips them, because the plugin runs them.

Add these lines to `.gitignore`. The first line ignores the run state of the harness. The second line keeps the plugin in git:

```text
.harness/*
!.harness/plugins/
```

## Step 3: run the gates two times

Commit the files before you run `harness verify`. The first run executes the setup step `go-mod`, and then each gate in the container.
The setup step prints `setup go-mod <duration>` before the summary line. The second run changes no input. The setup step prints nothing, and the summary shows `cached=4`:

```text
$ git add -A && git commit -q -m "chore: add the harness"
$ harness verify
setup go-mod 307ms
PASS gates=4 cached=0
$ harness verify
PASS gates=4 cached=4
```

`verify.parallel: 2` lets the harness run two gates at the same time. `go-fmt` and `go-vet` have no `needs`, so they start together.
`go-test` has `needs: [go-fmt, go-vet]`. It starts only after both gates pass or are cached.

## A failed gate skips the gates that need it

Break the format of a file. This run uses `internal/order/application/place_order/handler.go` with one line that has no indent:

```text
$ harness verify
FAIL go-fmt 540ms
SKIPPED go-test needs go-fmt

go-fmt:
 Container my-go-app-go-run-42b0a8c40957 Creating 
 Container my-go-app-go-run-42b0a8c40957 Created 
internal/order/application/place_order/handler.go

FAIL gates=4 failed=1
error: gates failed
$ echo $?
1
```

`go-fmt` fails and prints the name of the file. `go-vet` does not need `go-fmt`, so it runs and passes. `go-test` needs `go-fmt`, so the harness skips it. `lint` runs again, because it hashes all the files, and it passes. The harness prints no line for a passed gate. `harness verify` exits with code 1.
The setup step does not run, because no file of its `paths` changed.

Restore the file. The inputs return to the state of the last pass, so each gate is cached:

```text
$ git checkout internal/order/application/place_order/handler.go
Updated 1 path from the index
$ harness verify
PASS gates=4 cached=4
```

## A new dependency runs the setup step again

The setup step has the `paths` `go.mod`, `go.sum` and `compose.yaml`. A change to one of them runs the step again.
This run adds `go-cmp` to the module, so `go.mod` changes. `go.mod` is also in the `paths` of the gates, so the gates run again too:

```text
$ docker compose run --rm -T go go get github.com/google/go-cmp@v0.7.0
 Container my-go-app-go-run-b6b75a55528e Creating 
 Container my-go-app-go-run-b6b75a55528e Created 
go: added github.com/google/go-cmp v0.7.0
$ git diff --stat
 go.mod | 2 ++
 1 file changed, 2 insertions(+)
$ harness verify
setup go-mod 313ms
PASS gates=4 cached=0
$ harness verify
PASS gates=4 cached=4
```

The next run prints no setup line. The step runs only when the hash of its `paths` changes.

`harness verify --commits` checks each commit in a temporary worktree. The gates run in the same container, with the same cache volumes. The setup step runs there only when the `go.mod`, `go.sum` or `compose.yaml` of the commit differ from its last pass:

```text
$ git add -A && git commit -q -m "chore: add go-cmp"
$ harness verify --commits HEAD~1
base HEAD~1 0a83514a6119
PASS commits=1
```

## Architecture rules with go-arch-lint

The `lint-setup` skill wrote the config, the plugin and the pack with its steps. `harness lint catalogue` prints the rules of the catalogue.
The project does not turn on the built-in `ddd` pack, because its engine reads TypeScript.
The custom pack `architecture` gives four rules of the `ddd` pack to the engine `go-arch-lint`. Its scope is `internal/**/*.go`.
The one waived rule is `ddd/file-suffix`. go-arch-lint checks the imports of a package, not the names of the files, so the custom pack has no rule for it.

The skill proved the rules once at the setup. It wrote a temporary proof folder `lint/proof/` with one canary for each rule, and it ran `harness lint canaries --proof lint/proof`. Each canary breaks its rule on purpose.
The skill reported each PASS and each WAIVED line with its reason to the user, and then it deleted the folder.
The project commits no canary, no manifest and no canary gate. A later edit of `.go-arch-lint.yml` that lets a rule through stays green until someone runs the `lint-setup` skill again.

`.go-arch-lint.yml` has one component for each layer of each context, for example `order-domain` for `internal/order/domain`. The component `shared-domain` covers `internal/shared/domain`, where the integration event interfaces live. Each layer may use it, and it uses nothing.
The `deps` key lets a domain use only the shared domain.
It lets the application layer use its domain, the shared domain and any third-party library.
It lets the infrastructure layer use its domain, its application layer, the shared domain and any third-party library.
The key `depOnAnyVendor: false` makes a third-party library a violation for each component without `anyVendorDeps: true`, so only the domain cannot use one. The standard library is always allowed. Test files are not checked.
The `in` globs end with `/**`, so a component also covers the use case folders below its layer folder. A component never lists a component of another context, so each context stays apart. The shared domain is not a context.
A `.go` file under `internal/` that no component covers is a `ddd/src-roots` finding, for example a third context or a file directly in `internal/order/`.
Edit the components and the `deps` to match your project. Then run the `lint-setup` skill again to prove the rules.

To add a context, for example `catalog`, add three components and their `deps` to `.go-arch-lint.yml`:

```yaml
components:
  catalog-domain: {in: catalog/domain/**}
  catalog-application: {in: catalog/application/**}
  catalog-infrastructure: {in: catalog/infrastructure/**}
deps:
  catalog-domain:
    mayDependOn: [shared-domain]
  catalog-application:
    mayDependOn: [catalog-domain, shared-domain]
    anyVendorDeps: true
  catalog-infrastructure:
    mayDependOn: [catalog-domain, catalog-application, shared-domain]
    anyVendorDeps: true
```

The plugin `go-arch-lint.sh` does these steps:

1. It reads the event from stdin before it starts the container.
2. It reads the folder of the call with `git rev-parse --show-prefix`. The harness runs the plugin in a canary folder for a proof, and in the project root for `harness lint`.
3. It runs `go-arch-lint check --json` in a container with the name `go-arch-lint-<pid>`, with `--project-path` set to that folder. The config is always `/src/.go-arch-lint.yml`. A proof canary folder has its own `go.mod` with the module path `example.com/shop`, and its imports use this path, so the tool resolves the imports. A canary folder holds every component folder, because the tool fails with `not found directories` for a missing folder. The folders without code hold an empty `.gitkeep`. No `.go` file sits directly in an `application/` folder: put the code in `application/place_order/` or `application/issue_invoice/`.
4. A missing config gives no JSON, so the plugin prints the tool text to stderr and exits with 1. A config with a warning stops the plugin with an error that names the warning.
5. It gives the event and the report to `jq`. An import of another context is a `ddd/cross-context` finding. A violation of a `domain` component is a `ddd/domain-purity` finding. A violation of an `application` component is a `ddd/layer-direction` finding. Any other violation has no catalogue rule, for example a component `order-api` that you add. The plugin then stops with an error that names the violation.
6. Each file in `ArchWarningsNotMatched` of the report is a `ddd/src-roots` finding at line 1. go-arch-lint checks no import of such a file, so add a component for it.
7. It keeps only the files and the rules of the event.
8. A plugin timeout sends SIGTERM to the plugin. The signal stops the `docker compose` client, but not the container. The plugin traps SIGTERM and runs `docker kill` with the container name. The `--rm` flag then removes the container.

The proof set the plugin `timeout` to `1s`. No lint container stayed on the host:

```console
$ harness lint

error: lint engine go-arch-lint: plugin failed: go-arch-lint: context deadline exceeded: exit status 143
$ echo $?
1
$ docker ps -a --filter name=-lint- --format '{{.Names}}'
```

A domain file that imports a third-party library gives a finding. The proof added `internal/order/domain/identity.go` that imports `github.com/google/uuid`:

```console
$ harness lint
error ddd/domain-purity internal/order/domain/identity.go:3 order-domain must not depend on github.com/google/uuid
error: lint found error findings
$ echo $?
1
```

A context that `.go-arch-lint.yml` does not list gives a `ddd/src-roots` finding for each file. The proof added a context `shipping`: its domain imports the order domain, and its application layer imports its infrastructure layer:

```console
$ harness lint
error ddd/src-roots internal/shipping/application/ship/handler.go:1 internal/shipping/application/ship/handler.go is in no component of .go-arch-lint.yml
error ddd/src-roots internal/shipping/domain/shipment.go:1 internal/shipping/domain/shipment.go is in no component of .go-arch-lint.yml
error ddd/src-roots internal/shipping/infrastructure/carrier.go:1 internal/shipping/infrastructure/carrier.go is in no component of .go-arch-lint.yml
error: lint found error findings
$ echo $?
1
```

After the proof added the three `shipping` components and their `deps`, go-arch-lint checked the imports of the files:

```console
$ harness lint
error ddd/layer-direction internal/shipping/application/ship/handler.go:3 shipping-application must not depend on example.com/shopqa/internal/shipping/infrastructure
error ddd/cross-context internal/shipping/domain/shipment.go:3 shipping-domain must not depend on example.com/shopqa/internal/order/domain
error: lint found error findings
$ echo $?
1
```

A violation with no catalogue rule stops the plugin. The proof added the component `order-api: {in: order/api/**}` with no `deps` entry, and `internal/order/api/api.go` that imports the order domain:

```console
$ harness lint
jq: error (at <stdin>:1): go-arch-lint violation with no catalogue rule: order-api must not depend on example.com/shopqa/internal/order/domain
error: lint engine go-arch-lint: plugin failed: go-arch-lint: exit status 5
$ echo $?
1
```

## The project after the setup

The marks on the right are not part of a `tree` output. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.
The project keeps no `lint/proof/` folder, because the skill deletes it after the proof.

```text
.
├── .claude
│   └── settings.json                                                                           harness writes (host install), commit
├── .harness
│   └── plugins
│       └── go-arch-lint.sh                                                                     you add, commit
├── .harness-lint
│   ├── VERSION                                                                                 harness writes (lint init), commit
│   └── baseline.json                                                                           harness writes (lint baseline), commit
├── cmd
│   └── app
│       └── main.go
├── internal
│   ├── billing
│   │   ├── application
│   │   │   └── issue_invoice
│   │   │       └── handler.go
│   │   ├── domain
│   │   │   └── invoice.go
│   │   └── infrastructure
│   │       └── doc.go
│   └── order
│       ├── application
│       │   └── place_order
│       │       ├── handler.go
│       │       └── handler_test.go
│       ├── domain
│       │   └── order.go
│       └── infrastructure
│           └── memory_repository.go
├── lint
│   └── architecture
│       └── pack.yaml                                                                           you add, commit
├── .gitignore                                                                                  you add the harness lines, commit
├── .go-arch-lint.yml                                                                           you add, commit
├── .harness.yaml                                                                               you add the plugin, the pack and the lint gate, commit
├── compose.yaml                                                                                you add, commit
├── go.mod
└── go.sum                                                                                      go writes, commit
```

The harness keeps the state of `harness verify` in `.git/.harness/state.db`. The `.harness/` folder of the tree holds only the plugin. The harness writes its run folders there when you run `harness run`. `.gitignore` ignores them, except `.harness/plugins/`.
The harness writes `.claude/settings.json`, `.harness-lint/VERSION` and `.harness-lint/baseline.json` when you run `harness init --host claude`.
`go.sum` appeared after the proof added the dependency `go-cmp`. A module with no dependency has no `go.sum`.

## The gate paths

The input hash of a gate covers the files that match its `paths`:

- `**/*.go`: the Go sources.
- `**/testdata/**`, `go.work` and `go.work.sum`: the other files that `go vet` and `go test` read. Add the files that a `//go:embed` line reads, if they are not in `testdata/`.
- `go.mod` and `go.sum`: the dependencies. Commit `go.sum`. A module with no dependency has no `go.sum`, and the path then matches nothing.
- `compose.yaml`: the image is not in the input hash, so a change of the image must change a file in the hash. Change the image in `compose.yaml` and the gates run again.

The setup step `go-mod` has the `paths` `go.mod`, `go.sum` and `compose.yaml`. Its hash does not cover the `.go` files.

A file that a gate reads but that its `paths` do not match does not run the gate again. Then `harness verify` counts a stale result in `cached`.

The harness leaves out the files that `.gitignore` lists. Do not ignore a file that a gate needs. `harness init` adds `.harness/runs/` to `.gitignore`, which holds the run folders. That rule keeps `.harness/plugins/` in git.

## Where each part runs

- The host runs `harness`, the hooks, the guard and the plugin script `go-arch-lint.sh`.
- The container runs `gofmt`, `go vet`, `go test` and go-arch-lint.
- The named volumes `go-mod` and `go-build` keep the caches between runs.

On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
