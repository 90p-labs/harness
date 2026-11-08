# Next.js and shadcn example

This example shows how a Next.js App Router project with shadcn sets up the harness.
The lint packs are `nextjs` and `shadcn`.
The setup step `node-install` and the gates `node-typecheck`, `node-lint` and `node-build` run in a Docker Compose service.
The gate `lint` runs on the host.

## Files

The example holds only the files that you add to your project:

| File | Purpose |
|---|---|
| `Dockerfile` | A pinned `node` image with a pinned `pnpm`. |
| `compose.yaml` | The service `node`. It mounts the project and keeps `node_modules` in a named volume. |
| `.harness.yaml` | The keys `lint`, `verify`, `setup` and `gates`. |

The project is not in this folder. The example was tested with Next.js 16.4.5 (`create-next-app@16.4.5`) and `shadcn@4.23.0`.

Set the top-level `name:` of `compose.yaml` to the name of your project.
`harness verify --commits` runs the gates in a temporary worktree.
The fixed name lets that worktree use the same volume and the same image.
A second checkout that runs gates at the same time, such as a git worktree for a second agent, must not share the `node_modules` volume.
Two installs into one volume can break each other.
Export `COMPOSE_PROJECT_NAME` with a different value in the shell of that checkout. The variable overrides `name:`.
The harness keeps one setup pass for the repo, so it can skip the setup step in that checkout, and its new volume stays empty.
Run `docker compose run --rm -T node pnpm install --frozen-lockfile` once in that checkout.
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
- The setup step `node-install` and the gates `node-typecheck`, `node-lint` and `node-build` run in the container with `docker compose run --rm -T node`. The `-T` flag turns off the TTY, because the harness runs a gate with no terminal.
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
The setup step `node-install` runs `pnpm install --frozen-lockfile` in the container.
`harness verify` runs the setup before the gates.
A `pnpm-lock.yaml` change runs the step again and installs the new versions.
The temporary worktree of `harness verify --commits` uses the same `node_modules` volume.
The setup step runs in the worktree when the step `paths` of the checked commit differ from its last pass, so the install puts the versions of that commit into the volume.
The gate `node-typecheck` runs `next typegen` before `tsc`. Without it, `tsc` does not know the global type `LayoutProps`.

The gates `node-typecheck` and `node-lint` have the same `paths`:

- `**/*.ts`, `**/*.tsx`, `**/*.js`, `**/*.jsx`, `**/*.mjs`, `**/*.cjs`, `**/*.mts`, `**/*.cts` and `**/*.json`: the sources and the configs, in `src/` or in `app/`, `components/` and `lib/`.
- `.npmrc`, `pnpm-lock.yaml` and `pnpm-workspace.yaml`: the dependencies and the pnpm settings. Commit `pnpm-lock.yaml`.
- `compose.yaml` and `Dockerfile`: the harness does not hash the image, so an image change must change a file in the hash.

The gate `node-build` adds `**/*.css` and `public/**`, because the build reads the styles and the public files.
The `paths` of the step `node-install` are `.npmrc`, `package.json`, `pnpm-lock.yaml`, `pnpm-workspace.yaml`, `compose.yaml` and `Dockerfile`.

The input hash holds the files that git tracks and the untracked files that `.gitignore` does not list.
`node_modules/` and `.next/` are in the `.gitignore` of the app, so they are not in the hash.
On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
Add `.harness/` to the `.gitignore`. It holds the run folders.

### 3. Set up the harness

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

### 4. Verify two times

The first run executes the setup step and the gates.
The second run reads the cache, because no input changed.
The second run prints no `setup` line, because the hash of the step `paths` did not change.

```console
$ harness verify
setup node-install 12.649s
PASS node-typecheck 6.953s
PASS node-lint 6.467s
PASS lint 441ms
PASS node-build 12.166s
$ harness verify
CACHED node-typecheck 6.953s
CACHED node-lint 6.467s
CACHED lint 440ms
CACHED node-build 12.165s
```

The three keys that shape this run:

- `setup` holds the install step. The harness runs it before the gates, and prints `setup <id> <duration>` when it runs. A step runs only when the hash of its `paths` changed since its last pass, or when its `creates` file is missing.
- `verify.parallel: 3` lets the harness run three gates at the same time. The gates `node-typecheck`, `node-lint` and `lint` start together.
- `needs` on `node-build` makes the gate wait. `node-build` starts only after `node-typecheck`, `node-lint` and `lint` are `PASS` or `CACHED`.

The gates print in the order of the file.

### 5. Change the lockfile

The setup step runs again when the hash of its `paths` changes.
The proof appended the line `# edit` to `pnpm-lock.yaml`, then ran `harness verify` two times.
The first run installs again and the gates run again, because `pnpm-lock.yaml` is in their `paths`.
The second run changes nothing, so it prints no `setup` line.

```console
$ echo "# edit" >> pnpm-lock.yaml
$ harness verify
setup node-install 4.517s
PASS node-typecheck 6.027s
PASS node-lint 5.991s
PASS lint 471ms
PASS node-build 7.764s
$ harness verify
CACHED node-typecheck 6.027s
CACHED node-lint 5.99s
CACHED lint 471ms
CACHED node-build 7.763s
$ git checkout pnpm-lock.yaml
```

### 6. Break a gate

The proof added `src/lib/bad-hook.tsx`. It calls a React hook inside an `if`, and ESLint reports an error.
The listing leaves out the BuildKit lines that `docker compose run` prints before the `pnpm` output.

```console
$ harness verify; echo "exit $?"
setup node-install 4.052s
PASS node-typecheck 6.01s
FAIL node-lint 5.874s
PASS lint 440ms
SKIPPED node-build needs node-lint

node-lint:
 Container my-next-app-node-run-da7132c403c9 Creating 
 Container my-next-app-node-run-da7132c403c9 Created 

> app@0.1.0 lint /app
> eslint


/app/src/lib/bad-hook.tsx
  5:5  error  React Hook "useEffect" is called conditionally. React Hooks must be called in the exact same order in every component render  react-hooks/rules-of-hooks

✖ 1 problem (1 error, 0 warnings)

 ELIFECYCLE  Command failed with exit code 1.


error: gates failed
exit 1
```

`node-lint` fails, so the harness does not run `node-build`. It prints `SKIPPED node-build needs node-lint`, and `harness verify` exits with 1.
The step `node-install` ran in this run, because step 5 put `pnpm-lock.yaml` back to the committed file.
Delete `src/lib/bad-hook.tsx`. The next `harness verify` reads the cache again:

```console
$ rm src/lib/bad-hook.tsx
$ harness verify; echo "exit $?"
CACHED node-typecheck 6.953s
CACHED node-lint 6.467s
CACHED lint 440ms
CACHED node-build 12.165s
exit 0
```

### 7. Verify a commit

`harness verify --commits` checks each commit in a temporary worktree. The proof added `src/lib/total.ts` in one commit:

```console
$ git commit -q -m "feat: add the cart total"
$ harness verify --commits HEAD~1
PASS 68cb9394c4cd feat
```

## The project after the setup

The tree comes from `tree -a -I "vendor|node_modules|.git|.venv|var|.next" --dirsfirst` in the project of the proof.
The pattern also leaves out `.next/`, because `next build` writes it and `.gitignore` ignores it.
The marks on the right are not part of the `tree` output. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.

```text
.
├── .claude
│   └── settings.json        harness writes (host install), commit
├── .harness-baseline
│   └── lint.json            harness writes (lint baseline), commit
├── .harness-lint
│   ├── biome
│   │   └── sd642062c
│   │       └── biome.json   harness writes (lint init), commit
│   ├── plugins
│   │   └── shadcn
│   │       ├── cn-classnames.grit   harness writes (lint init), commit
│   │       └── no-raw-colors.grit   harness writes (lint init), commit
│   ├── VERSION              harness writes (lint init), commit
│   └── oxlintrc.json        harness writes (lint init), commit
├── public
│   ├── file.svg
│   ├── globe.svg
│   ├── next.svg
│   ├── vercel.svg
│   └── window.svg
├── src
│   ├── app
│   │   ├── favicon.ico
│   │   ├── globals.css
│   │   ├── layout.tsx
│   │   └── page.tsx
│   ├── components
│   │   └── ui
│   │       └── button.tsx
│   └── lib
│       ├── total.ts
│       └── utils.ts
├── .gitignore               you add the harness line, commit
├── .harness.yaml            you add, commit
├── AGENTS.md
├── Dockerfile               you add, commit
├── README.md
├── components.json
├── compose.yaml             you add, commit
├── eslint.config.mjs
├── next-env.d.ts            ignore
├── next.config.ts
├── package.json
├── pnpm-lock.yaml
├── pnpm-workspace.yaml
├── tsconfig.json
└── tsconfig.tsbuildinfo     ignore

13 directories, 34 files
```

`next-env.d.ts` and `tsconfig.tsbuildinfo` are not harness files. `.gitignore` of the app ignores them.
The harness keeps the state of `harness verify` in `.git/.harness/state.db`, so the tree has no `.harness/` folder. `.harness/` appears when you run `harness run`, and the `.gitignore` line `.harness/` ignores it.
`node_modules/` is a mount point on the host. The packages are in the named volume.
