# Next.js and shadcn example

This example shows how a Next.js App Router project with shadcn sets up the harness.
The lint packs are `nextjs` and `shadcn`.
The gates `node-typecheck` and `node-lint` run in a Docker Compose service.
The gate `lint` runs on the host.

## Files

The example holds only the files that you add to your project:

| File | Purpose |
|---|---|
| `Dockerfile` | A pinned `node` image with a pinned `pnpm`. |
| `compose.yaml` | The service `node`. It mounts the project and keeps `node_modules` in a named volume. |
| `.harness.yaml` | The lint packs and the three gates. |

The project is not in this folder. The example was tested with Next.js 16.4.5 (`create-next-app@16.4.5`) and `shadcn@4.23.0`.

Set the top-level `name:` of `compose.yaml` to the name of your project.
`harness verify --commits` runs the gates in a temporary worktree.
The fixed name lets that worktree use the same volume and the same image.
A second checkout that runs gates at the same time, such as a git worktree for a second agent, must not share the `node_modules` volume.
Two installs into one volume can break each other.
Export `COMPOSE_PROJECT_NAME` with a different value in the shell of that checkout. The variable overrides `name:`.
A `.env` file is not enough, because the temporary worktree of `harness verify --commits` does not hold an untracked `.env`.

## Prerequisites

1. Install Docker and Docker Compose. The tests used Docker 29.9.2 and Compose v5.6.1.
2. Put the `harness` binary on the `PATH`.
3. Put Biome and oxlint on the host `PATH`:

```sh
npm i -g @biomejs/biome@2.6.6 oxlint@1.92.0
```

Do not add Biome to the `devDependencies` of the project.
The command `harness lint` looks for Biome in `node_modules/.bin` first.
A Linux Biome in that folder fails on a macOS host.
Create the app with ESLint, as this example does.

## What runs where

- The harness, the hooks and the gate `lint` run on the host.
- The gates `node-typecheck` and `node-lint` run in the container with `docker compose run --rm -T node`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.
- `node_modules` is a named volume. It is not on the host, so the host holds no Linux binary.
- The pnpm store is in the same volume, at `node_modules/.pnpm-store`. pnpm needs the store and `node_modules` on one file system. With the store in another place, pnpm wrote a 563 MB `.pnpm-store/` into the project.
- `pull_policy: build` builds the image at each `docker compose run`. The build cache makes this fast, and a `Dockerfile` change takes effect at the next gate.

## Build the image

Run the build before the first `harness verify`. A pull inside the gate timeout can record a failure.

```console
$ docker compose build
```

A first build downloads the base image: 80 MB of compressed layers on `linux/arm64` and 81 MB on `linux/amd64`, as the registry manifest gives them.
Then it installs pnpm from the npm registry. The time depends on your network.
The image is 398 MB on disk.
On the test host, `docker build --no-cache` took 6.4 s, because the base layers came from the local build cache.

## Steps

The paths in the output below show `/path/to/app` for the project folder.

### 1. See what the detection writes

On a bare app with `pnpm-lock.yaml`, `harness init --host claude` writes host commands, because it does not know about the container.
The app has no `typecheck` script, so the detection finds no `node-typecheck` gate.

```console
$ harness init --host claude
lint packs: wrote nextjs, shadcn
gates: wrote node-lint, lint
lint init: wrote .harness-lint
lint baseline: wrote .harness-baseline/lint.json
host: wrote /path/to/app/.claude/settings.json
$ cat .harness.yaml
lint:
  packs:
    - nextjs
    - shadcn
gates:
  - id: node-lint
    run: pnpm run lint
  - id: lint
    run: harness lint --json
    baseline: .harness-baseline/lint.json
```

Remove the files that this run wrote before the next step: `.harness.yaml`, `.harness-lint/`, `.harness-baseline/` and `.claude/`.

### 2. Copy the example

```console
$ cp /path/to/harness/examples/nextjs/compose.yaml /path/to/harness/examples/nextjs/Dockerfile /path/to/harness/examples/nextjs/.harness.yaml .
```

The gate ids `node-typecheck`, `node-lint` and `lint` are the ids of the built-in detection.
Each container gate runs `pnpm install --frozen-lockfile` first.
A `pnpm-lock.yaml` change then installs the new versions.
The temporary worktree of `harness verify --commits` uses the same `node_modules` volume, and the install puts the versions of the checked commit into it.
The gate `node-typecheck` runs `next typegen` before `tsc`. Without it, `tsc` does not know the global type `LayoutProps`.

The container gates have the same `paths`:

- `**/*.ts`, `**/*.tsx`, `**/*.js`, `**/*.jsx`, `**/*.mjs`, `**/*.cjs`, `**/*.mts`, `**/*.cts` and `**/*.json`: the sources and the configs, in `src/` or in `app/`, `components/` and `lib/`.
- `.npmrc`, `pnpm-lock.yaml` and `pnpm-workspace.yaml`: the dependencies and the pnpm settings. Commit `pnpm-lock.yaml`.
- `compose.yaml` and `Dockerfile`: the harness does not hash the image, so an image change must change a file in the hash.

The input hash holds the files that git tracks and the untracked files that `.gitignore` does not list.
`node_modules/` and `.next/` are in the `.gitignore` of the app, so they are not in the hash.
On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
Add `.harness/` to the `.gitignore`. It holds the run folders.

### 3. Install the dependencies in the container

This step is optional, because each gate installs the dependencies. The `git status` after it shows that the install wrote nothing into the project.

```console
$ docker compose run --rm -T node pnpm install --frozen-lockfile
 Image my-next-app-node Building 
 Image my-next-app-node Built 
 Container my-next-app-node-run-a215150fc2f4 Creating 
 Container my-next-app-node-run-a215150fc2f4 Created 
Lockfile is up to date, resolution step is skipped
Packages: +513
Done in 5.7s using pnpm v10.34.9
$ git status --porcelain
```

The listing leaves out the lines that create the network and the volume, the progress lines and the package list.

### 4. Set up the harness

`.harness.yaml` has `lint.packs`, so `harness init` keeps the packs and the gates.
It writes the lint files, the lint baseline and the hooks.

```console
$ harness init --host claude
lint packs: kept nextjs, shadcn
gates: kept
lint init: wrote .harness-lint
lint baseline: wrote .harness-baseline/lint.json
host: wrote /path/to/app/.claude/settings.json
```

Commit `.harness.yaml`, `.harness-lint/`, `.harness-baseline/` and `.claude/`.

### 5. Verify two times

The first run executes the gates.
The second run reads the cache, because no input changed.

```console
$ harness verify
PASS node-typecheck 3.994s
PASS node-lint 3.218s
PASS lint 208ms
$ harness verify
CACHED node-typecheck 3.994s
CACHED node-lint 3.218s
CACHED lint 207ms
```

### 6. Verify a commit

`harness verify --commits` checks each commit in a temporary worktree. The proof added `src/lib/total.ts` in one commit:

```console
$ git commit -q -m "feat: add the cart total"
$ harness verify --commits HEAD~1
PASS d1e4322e1047 feat
```
