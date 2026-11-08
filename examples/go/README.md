# Go example

This example sets up the harness on a Go module. Each gate runs in a Docker container. The harness and the host hooks run on the host.

## Files

The example holds only the files that you add to your module:

- `compose.yaml`: one service `go`. It uses the image `golang:1.27.3-bookworm`, pinned with its index digest. It mounts the project at `/src`. It keeps the module cache and the build cache in two named volumes, so a second run is fast.
- `.harness.yaml`: the `gates` key with `go-fmt`, `go-vet` and `go-test`. Each gate runs `docker compose run --rm -T go ...`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.

Set the top-level `name:` of `compose.yaml` to the name of your module. `harness verify --commits` runs the gates in a temporary worktree. The fixed name lets that worktree use the same cache volumes.

## Prerequisites

- Docker and Docker Compose. The output below comes from Docker 29.9.2 and Compose v5.6.1.
- The `harness` binary on the `PATH` of the host. See the root README for the install steps.
- A Go module with `go.mod` at the top of a git repository.

You do not need Go on the host. The container has Go.

## Start the container

Pull the image before the first `harness verify`. A pull inside a gate counts against the gate timeout.

```text
$ time docker compose pull
 Image golang:1.27.3-bookworm@sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0 Pulling 
 Image golang:1.27.3-bookworm@sha256:dc0963f32053e4bc451d24d87e3892d42d01021b61f093e98eb95c0352bb84b0 Pulled 
        1.26 real         0.05 user         0.06 sys
$ docker image ls golang
REPOSITORY   TAG       IMAGE ID       CREATED      SIZE
golang       <none>    dc0963f32053   3 days ago   1.25GB
```

The listing leaves out one row for an unrelated `golang` image on the test host.

A first pull downloads 290 MB of compressed layers on `linux/arm64` and 300 MB on `linux/amd64`, as the registry manifest gives them. The time depends on your network. The image is 1.25GB on disk. The pull above took 1.26 seconds, because the Docker store on the test host already held the layers.

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

Copy the two files to the top of your module, commit them, and run the setup:

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

The first run executes each gate in the container. The second run changes no input, so each gate is `CACHED`:

```text
$ harness verify
PASS go-fmt 420ms
PASS go-vet 3.916s
PASS go-test 1.703s
$ harness verify
CACHED go-fmt 419ms
CACHED go-vet 3.915s
CACHED go-test 1.703s
```

`harness verify --commits` checks each commit in a temporary worktree. The gates run there in the same container, with the same cache volumes:

```text
$ git commit -q -m "docs: describe the cart package"
$ harness verify --commits HEAD~1
PASS ae5ce7a72b60 docs
```

## The gate paths

The input hash of a gate covers the files that match its `paths`:

- `**/*.go`: the Go sources.
- `**/testdata/**`, `go.work` and `go.work.sum`: the other files that `go vet` and `go test` read. Add the files that a `//go:embed` line reads, if they are not in `testdata/`.
- `go.mod` and `go.sum`: the dependencies. Commit `go.sum`. A module with no dependency has no `go.sum`, and the path then matches nothing.
- `compose.yaml`: the image is not in the input hash, so a change of the image must change a file in the hash. Change the image in `compose.yaml` and the gates run again.

A file that a gate reads but that its `paths` do not match does not run the gate again. Then `harness verify` prints `CACHED` for a stale result.

The harness leaves out the files that `.gitignore` lists. Do not ignore a file that a gate needs. Add `.harness/` to `.gitignore`: it holds the run folders.

## Where each part runs

- The host runs `harness`, the hooks and the guard.
- The container runs `gofmt`, `go vet` and `go test`.
- The named volumes `go-mod` and `go-build` keep the caches between runs.

On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
