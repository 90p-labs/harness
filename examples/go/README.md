# Go example

This example sets up the harness on a Go module. Each gate runs in a Docker container. The harness and the host hooks run on the host.

## Files

The example holds only the files that you add to your module:

- `compose.yaml`: one service `go`. It uses the image `golang:1.27.3-bookworm`, pinned with its index digest. It mounts the project at `/src`. It keeps the module cache and the build cache in two named volumes, so a second run is fast.
- `.harness.yaml`: the keys `verify`, `setup` and `gates`.
  - `verify.parallel: 2` lets the harness run two gates at the same time.
  - `setup` holds one step, `go-mod`. It runs `go mod download` before the gates.
  - `gates` holds `go-fmt`, `go-vet` and `go-test`. `go-test` has `needs: [go-fmt, go-vet]`.
  - Each command runs `docker compose run --rm -T go ...`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.

Set the top-level `name:` of `compose.yaml` to the name of your module. `harness verify --commits` runs the gates in a temporary worktree. The fixed name lets that worktree use the same cache volumes.

## Prerequisites

- Docker and Docker Compose. The output below comes from Docker Engine 29.9.2 and Compose v5.6.1.
- The `harness` binary on the `PATH` of the host. See the root README for the install steps.
- A Go module with `go.mod` at the top of a git repository.

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

Copy the two files to the top of your module and run the setup:

```text
$ cp /path/to/harness/examples/go/compose.yaml /path/to/harness/examples/go/.harness.yaml .
$ harness init --host claude
lint packs: none detected
gates: kept
lint init: wrote .harness-lint
lint baseline: skipped, no lint pack
host: wrote /path/to/app/.claude/settings.json
```

The setup keeps the `gates` key of the example. `harness init` always installs the hooks. With the harness plugin, each hook then runs twice. See the root README for the setup with the plugin.

## Step 3: run the gates two times

Commit the files before you run `harness verify`. The first run executes the setup step `go-mod`, and then each gate in the container.
The setup step prints `setup go-mod <duration>` before the gate lines. The second run changes no input. The setup step prints nothing, and each gate is `CACHED`:

```text
$ git add -A && git commit -q -m "chore: add the harness"
$ harness verify
setup go-mod 1.462s
PASS go-fmt 578ms
PASS go-vet 1.087s
PASS go-test 1.148s
$ harness verify
CACHED go-fmt 578ms
CACHED go-vet 1.086s
CACHED go-test 1.148s
```

`verify.parallel: 2` lets the harness run two gates at the same time. `go-fmt` and `go-vet` have no `needs`, so they start together.
`go-test` has `needs: [go-fmt, go-vet]`. It starts only after both gates are `PASS` or `CACHED`.

## A failed gate skips the gates that need it

Break the format of a file. This run uses `cart/cart.go` with two lines that have no indent:

```text
$ harness verify
FAIL go-fmt 1.811s
PASS go-vet 2.055s
SKIPPED go-test needs go-fmt

go-fmt:
 Container my-go-app-go-run-61d03facbd1a Creating 
 Container my-go-app-go-run-61d03facbd1a Created 
cart/cart.go


error: gates failed
$ echo $?
1
```

`go-fmt` fails and prints the name of the file. `go-vet` does not need `go-fmt`, so it runs and passes. `go-test` needs `go-fmt`, so the harness skips it. `harness verify` exits with code 1.
The setup step does not run, because no file of its `paths` changed.

Restore the file. The inputs return to the state of the last pass, so each gate is `CACHED`:

```text
$ git checkout cart/cart.go
Updated 1 path from the index
$ harness verify
CACHED go-fmt 578ms
CACHED go-vet 1.086s
CACHED go-test 1.148s
```

## A new dependency runs the setup step again

The setup step has the `paths` `go.mod`, `go.sum` and `compose.yaml`. A change to one of them runs the step again.
This run changes the version of `go-cmp`, so `go.mod` and `go.sum` change. These two files are also in the `paths` of the gates, so the gates run again too:

```text
$ docker compose run --rm -T go go get github.com/google/go-cmp@v0.6.0
go: downloading github.com/google/go-cmp v0.6.0
go: downgraded github.com/google/go-cmp v0.7.0 => v0.6.0
$ git diff --stat
 go.mod | 2 +-
 go.sum | 2 ++
 2 files changed, 3 insertions(+), 1 deletion(-)
$ harness verify
setup go-mod 388ms
PASS go-fmt 560ms
PASS go-vet 885ms
PASS go-test 603ms
$ harness verify
CACHED go-fmt 559ms
CACHED go-vet 885ms
CACHED go-test 602ms
```

The next run prints no setup line. The step runs only when the hash of its `paths` changes.

`harness verify --commits` checks each commit in a temporary worktree. The gates run in the same container, with the same cache volumes. The setup step runs there only when the `go.mod`, `go.sum` or `compose.yaml` of the commit differ from its last pass:

```text
$ git add -A && git commit -q -m "chore: downgrade go-cmp"
$ harness verify --commits HEAD~1
PASS f911de379caf chore
```

## The project after the setup

The tree comes from `tree -a -I "vendor|node_modules|.git|.venv|var" --dirsfirst` in the project of the proof.
The marks on the right are not part of the `tree` output. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.

```text
.
├── .claude
│   └── settings.json        harness writes (host install), commit
├── .harness-lint
│   └── VERSION              harness writes (lint init), commit
├── cart
│   ├── cart.go
│   └── cart_test.go
├── .gitignore               you add the harness lines, commit
├── .harness.yaml            you add, commit
├── compose.yaml             you add, commit
├── go.mod
└── go.sum

3 directories, 9 files
```

The harness keeps the state of `harness verify` in `.git/.harness/state.db`, so the tree has no `.harness/` folder. `.harness/` appears when you run `harness run`. `.gitignore` ignores it.
The harness writes `.claude/settings.json` and `.harness-lint/VERSION` when you run `harness init --host claude`.

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

- The host runs `harness`, the hooks and the guard.
- The container runs `gofmt`, `go vet` and `go test`.
- The named volumes `go-mod` and `go-build` keep the caches between runs.

On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
