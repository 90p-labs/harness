# Next.js and shadcn example

This example shows how a Next.js App Router project with shadcn sets up the harness.
The lint packs are `nextjs`, `shadcn` and the built-in `ddd` pack.
The setup step `node-install` and the gates `node-typecheck`, `node-lint` and `node-build` run in a Docker Compose service.
The gate `lint` runs on the host.

## Files

The example holds only the files that you add to your project:

| File | Purpose |
|---|---|
| `Dockerfile` | A pinned `node` image with a pinned `pnpm`. |
| `compose.yaml` | The service `node`. It mounts the project and keeps `node_modules` in a named volume. |
| `.harness.yaml` | The keys `lint` (with the pack `ddd`), `verify`, `setup` and `gates`. |
| `lint/rules.yaml` | The manifest. It gives a canary to each of the five rules of the `ddd` catalogue pack. It is optional. |
| `lint/canaries/` | One folder for each rule. Each folder breaks its rule on purpose. |

The project is not in this folder. The example was tested with Next.js 16.4.5 (`create-next-app@16.4.5`) and `shadcn@4.23.0`.
The proof made the app with these commands:

```console
$ pnpm dlx create-next-app@16.4.5 app --ts --tailwind --eslint --app --src-dir --import-alias "@/*" --use-pnpm --disable-git --yes
$ cd app
$ pnpm dlx shadcn@4.23.0 init --defaults --yes
$ pnpm dlx shadcn@4.23.0 add button --yes
$ cp -R /path/to/contexts src/
$ rm -rf node_modules
$ git init -q && git add -A && git commit -qm "feat: start the shop"
```

The `cp` command adds the business code in `src/contexts/`. Step 3 tells what it holds.

shadcn needs Tailwind. An app made with `--no-tailwind` cannot run `shadcn init`.
`--eslint` gives the gate `node-lint`, and `--src-dir` puts the code in `src/`, where the `ddd` pack looks for the business root.
The `rm` command removes the host `node_modules`, because the packages go into a named volume.

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
The detection chooses `nextjs` and `shadcn`. It does not choose `ddd` for a Next.js app, and it prints no `next: run the lint-setup skill` line for TypeScript.

```console
$ harness init --host claude
lint packs: wrote nextjs, shadcn
gates: wrote node-lint, lint
lint init: wrote .harness-lint
lint baseline: wrote .harness-baseline/lint.json (10 findings, run harness lint to see them)
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
$ cp -R /path/to/harness/examples/nextjs/lint .
```

The `lint` folder holds the optional canaries. Without it, remove the gate `lint-canaries` from `.harness.yaml`, and remove `lint-canaries` from the `needs` of `node-build`. The section [Canaries](#canaries) tells what they prove.

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
lint packs: kept nextjs, shadcn, ddd
gates: kept
lint init: wrote .harness-lint
lint baseline: wrote .harness-baseline/lint.json (10 findings, run harness lint to see them)
host: wrote /path/to/app/.claude/settings.json
$ harness lint canaries
PASS ddd/src-roots lint/canaries/ddd-src-roots
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
PASS ddd/layer-direction lint/canaries/ddd-layer-direction
PASS ddd/cross-context lint/canaries/ddd-cross-context
PASS ddd/file-suffix lint/canaries/ddd-file-suffix
```

The `ddd` pack reads the business root `src/contexts/`. The proof app has two contexts, `cart` and `catalog`, each with the folders `domain`, `application` and `infrastructure`, and the file `src/contexts/kernel.ts`.
The baseline accepts the findings of the new app.
shadcn creates `src/lib/utils.ts`. The rules `ddd/src-roots` and `next/no-lib-folder` refuse the folder `src/lib`.
Move the shadcn utils file to `src/components/shared/utils.ts`, and set the `utils` alias in `components.json` to `@/components/shared/utils`.
`harness lint` gives no finding in `lint/canaries/`, so the canaries do not change the baseline.
Commit `.harness.yaml`, `lint/`, `.harness-lint/`, `.harness-baseline/` and `.claude/`.

### 4. Verify two times

The first run executes the setup step and the gates.
The second run reads the cache, because no input changed.
The second run prints no `setup` line, because the hash of the step `paths` did not change.

```console
$ harness verify
setup node-install 3.064s
PASS node-typecheck 5.112s
PASS node-lint 4.889s
PASS lint 401ms
PASS lint-canaries 142ms
PASS node-build 8.944s
$ harness verify
CACHED node-typecheck 5.111s
CACHED node-lint 4.889s
CACHED lint 400ms
CACHED lint-canaries 142ms
CACHED node-build 8.943s
```

The three keys that shape this run:

- `setup` holds the install step. The harness runs it before the gates, and prints `setup <id> <duration>` when it runs. A step runs only when the hash of its `paths` changed since its last pass, or when its `creates` file is missing.
- `verify.parallel: 3` lets the harness run three gates at the same time. The gates `node-typecheck`, `node-lint` and `lint` start together, and `lint-canaries` starts when one of them ends.
- `needs` on `node-build` makes the gate wait. `node-build` starts only after `node-typecheck`, `node-lint`, `lint` and `lint-canaries` are `PASS` or `CACHED`.

The gates print in the order of the file.

### 5. Change the lockfile

The setup step runs again when the hash of its `paths` changes.
The proof appended the line `# edit` to `pnpm-lock.yaml`, then ran `harness verify` two times.
The first run installs again and the gates run again, because `pnpm-lock.yaml` is in their `paths`.
The second run changes nothing, so it prints no `setup` line.

```console
$ echo "# edit" >> pnpm-lock.yaml
$ harness verify
setup node-install 3.247s
PASS node-typecheck 4.983s
PASS node-lint 4.81s
PASS lint 442ms
PASS lint-canaries 213ms
PASS node-build 6.557s
$ harness verify
CACHED node-typecheck 4.982s
CACHED node-lint 4.81s
CACHED lint 441ms
CACHED lint-canaries 213ms
CACHED node-build 6.557s
$ git checkout pnpm-lock.yaml
```

### 6. Break a gate

The proof added `src/components/bad-hook.tsx`. It calls a React hook inside an `if`, and ESLint reports an error.
The listing leaves out the BuildKit lines that `docker compose run` prints before the `pnpm` output.
The proof exported `COMPOSE_PROJECT_NAME=nextproof`, so the container name starts with `nextproof`, not `my-next-app`.

```console
$ harness verify; echo "exit $?"
setup node-install 2.845s
PASS node-typecheck 4.566s
FAIL node-lint 4.389s
PASS lint 579ms
PASS lint-canaries 141ms
SKIPPED node-build needs node-lint

node-lint:
 Container nextproof-node-run-07102f8a508d Creating 
 Container nextproof-node-run-07102f8a508d Created 

> app@0.1.0 lint /app
> eslint


/app/src/components/bad-hook.tsx
  5:5  error  React Hook "useEffect" is called conditionally. React Hooks must be called in the exact same order in every component render  react-hooks/rules-of-hooks

✖ 1 problem (1 error, 0 warnings)

 ELIFECYCLE  Command failed with exit code 1.


error: gates failed
exit 1
```

`node-lint` fails, so the harness does not run `node-build`. It prints `SKIPPED node-build needs node-lint`, and `harness verify` exits with 1.
The step `node-install` ran in this run, because step 5 put `pnpm-lock.yaml` back to the committed file.
Delete `src/components/bad-hook.tsx`. The next `harness verify` reads the cache again:

```console
$ rm src/components/bad-hook.tsx
$ harness verify; echo "exit $?"
CACHED node-typecheck 5.111s
CACHED node-lint 4.889s
CACHED lint 400ms
CACHED lint-canaries 142ms
CACHED node-build 8.943s
exit 0
```

### 7. Verify a commit

`harness verify --commits` checks each commit in a temporary worktree. The proof added `src/contexts/cart/domain/cart-total.entity.ts` in one commit:

```console
$ git commit -q -m "feat: add the cart total"
$ harness verify --commits HEAD~1
PASS c2e66bbfdafd feat
```

## Architecture rules with the built-in ddd pack

The `lint-setup` skill has a TypeScript path. For TypeScript it picks the built-in `ddd` pack, because the `harness` binary already checks the layers, the contexts and the file suffixes.
The skill adds `ddd` to `lint.packs` and stops. This example shows what that gives:

- `.harness.yaml` has `packs: [nextjs, shadcn, ddd]`. The skill adds the one word `ddd`.
- No tool config, no wrapper plugin and no `lint/architecture/` pack.
- No canary is necessary. The tests of the `harness` binary prove each built-in rule with embedded fixtures.

The example uses `ddd` beside `nextjs` and `shadcn`. The pack `nextjs-ddd-shadcn` needs all three packs in `lint.packs`, and it adds only the rules for the borders between them.
The `ddd` pack holds 23 rules.

### Canaries

The canaries are optional. They prove that the rules still catch a violation with the `lint.ddd` keys of your project, such as `domain_allow`.
Without `lint/rules.yaml`, `harness lint canaries` prints one line and exits with code 0:

```console
$ harness lint canaries
lint canaries: only built-in rules are active, so there is no canary to run
$ echo $?
0
```

The example adds `lint/rules.yaml` and one canary for each of the five rules of the `ddd` catalogue pack:

- `ddd-src-roots` has `src/lib/utils.ts`. The folder `src/lib` is not `app`, `components` or the business root.
- `ddd-domain-purity` has `src/contexts/cart/domain/checkout-date.entity.ts`, which imports `version` from `react`.
- `ddd-layer-direction` has an application handler that imports `cart.memory-repository.ts` from the infrastructure.
- `ddd-cross-context` has a `cart` handler that imports `product.aggregate.ts` from the `catalog` domain.
- `ddd-file-suffix` has `src/contexts/cart/domain/cart.ts`, a domain file with no suffix.

Each canary is valid TypeScript, so the gates `node-typecheck`, `node-lint` and `node-build` pass with the canaries in the project. `harness lint` gives no finding in `lint/canaries/`.
A canary proves that the rule catches one fixture. It does not prove that the rule catches each violation in the project.

The proof let the domain use `react`:

```diff
 lint:
   packs: [nextjs, shadcn, ddd]
+  ddd:
+    domain_allow: [react]
```

The canary `ddd-domain-purity` then gave no finding, so the gate `lint-canaries` failed:

```console
$ time harness verify
CACHED node-typecheck 5.111s
CACHED node-lint 4.889s
PASS lint 483ms
FAIL lint-canaries 128ms
SKIPPED node-build needs lint-canaries

lint-canaries:
PASS ddd/src-roots lint/canaries/ddd-src-roots
FAIL ddd/domain-purity lint/canaries/ddd-domain-purity: the engine harness gave no ddd/domain-purity finding, and the built-in engine reads only TypeScript and JavaScript
PASS ddd/layer-direction lint/canaries/ddd-layer-direction
PASS ddd/cross-context lint/canaries/ddd-cross-context
PASS ddd/file-suffix lint/canaries/ddd-file-suffix
error: a lint canary failed

error: gates failed

real	0m0.575s
user	0m0.327s
sys	0m0.349s
```

The gate `lint` passes, because the code has no violation. The proof then restored `.harness.yaml`.

### A rule finds a domain file

The proof ran `docker compose run --rm -T node pnpm add date-fns`, then added `src/contexts/cart/domain/checkout-date.entity.ts`. It imports `addDays` from the package `date-fns`.
The domain may use only itself, the shared domain and the packages in `lint.ddd.domain_allow`, so `harness lint` gives a finding.
`harness lint` prints the findings of all packs. The listing keeps the `ddd` lines:

```console
$ harness lint
error ddd/domain-purity src/contexts/cart/domain/checkout-date.entity.ts:1 A domain file imports a package that is not allowed, or an application or infrastructure path.
error: lint found error findings
```

The `ddd/domain-purity` finding is new, so the gate `lint` fails and the harness skips `node-build`.

### Tune a built-in rule

Tune a rule in `.harness.yaml`, in the `lint.ddd` keys. The built-in rules have no other config file.

| Key | Effect |
|---|---|
| `root` | The business root under `src/`. The default is `contexts`. |
| `domain_allow` | The packages that the domain may import. |
| `src_files` | The files that may sit directly in `src/`. |

`lint.disabled` turns off a lint engine or one rule. An entry with a `/` is a rule id, such as `next/no-lib-folder`. An unknown rule id is an error.

The proof allowed `date-fns` in the domain:

```yaml
lint:
  packs: [nextjs, shadcn, ddd]
  ddd:
    domain_allow: [date-fns]
```

The finding goes away, and the gate passes:

```console
$ harness lint
$ harness verify
CACHED node-typecheck 4.617s
CACHED node-lint 4.535s
PASS lint 350ms
PASS lint-canaries 113ms
PASS node-build 5.954s
```

The listing keeps the `ddd` lines, and no `ddd` finding is left.

### Accept a finding

Some findings are not worth a fix now. Accept them in the baseline `.harness-baseline/lint.json`.
The proof added `src/contexts/cart/domain/legacy-total.entity.ts`. It imports a path in `infrastructure`, which `ddd/domain-purity` forbids.
The baseline only shrinks, so `harness lint baseline` does not add a finding. To accept the new finding, delete `.harness-baseline/lint.json` and write it again:

```console
$ rm .harness-baseline/lint.json
$ harness lint baseline
$ git diff --stat
 .harness-baseline/lint.json |  7 +++++++
 .harness.yaml               |  2 ++
 package.json                |  1 +
 pnpm-lock.yaml              | 13 +++++++++++--
 4 files changed, 21 insertions(+), 2 deletions(-)
$ harness verify
PASS node-typecheck 4.625s
PASS node-lint 4.25s
PASS lint 383ms
PASS lint-canaries 143ms
PASS node-build 5.993s
```

The diff also holds the `date-fns` change and the `domain_allow` key of the two sections above, because the proof had not committed them yet.

The new baseline holds all the findings of the moment, so read `harness lint` before you write it. Commit the file. A later `harness lint baseline` removes the finding when you fix the file.

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
│   │   ├── s02bf6b06
│   │   │   └── biome.json   harness writes (lint init), commit
│   │   ├── s17bd3bad
│   │   │   └── biome.json   harness writes (lint init), commit
│   │   ├── s41260697
│   │   │   └── biome.json   harness writes (lint init), commit
│   │   ├── s52a653d5
│   │   │   └── biome.json   harness writes (lint init), commit
│   │   ├── sd642062c
│   │   │   └── biome.json   harness writes (lint init), commit
│   │   └── sf9b553be
│   │       └── biome.json   harness writes (lint init), commit
│   ├── plugins
│   │   ├── ddd
│   │   │   ├── aggregate-encapsulation.grit   harness writes (lint init), commit
│   │   │   ├── command-handler-returns.grit   harness writes (lint init), commit
│   │   │   ├── event-readonly.grit   harness writes (lint init), commit
│   │   │   ├── publish-after-save.grit   harness writes (lint init), commit
│   │   │   ├── query-handler-no-write.grit   harness writes (lint init), commit
│   │   │   ├── value-object-equals.grit   harness writes (lint init), commit
│   │   │   └── value-object-factory.grit   harness writes (lint init), commit
│   │   └── shadcn
│   │       ├── cn-classnames.grit   harness writes (lint init), commit
│   │       └── no-raw-colors.grit   harness writes (lint init), commit
│   ├── VERSION              harness writes (lint init), commit
│   └── oxlintrc.json        harness writes (lint init), commit
├── lint
│   ├── canaries
│   │   ├── ddd-cross-context
│   │   │   └── src
│   │   │       └── contexts
│   │   │           ├── cart
│   │   │           │   └── application
│   │   │           │       └── commands
│   │   │           │           └── add-item
│   │   │           │               └── add-item.handler.ts   you add, commit
│   │   │           └── catalog
│   │   │               └── domain
│   │   │                   └── product.aggregate.ts   you add, commit
│   │   ├── ddd-domain-purity
│   │   │   └── src
│   │   │       └── contexts
│   │   │           └── cart
│   │   │               └── domain
│   │   │                   └── checkout-date.entity.ts   you add, commit
│   │   ├── ddd-file-suffix
│   │   │   └── src
│   │   │       └── contexts
│   │   │           └── cart
│   │   │               └── domain
│   │   │                   └── cart.ts   you add, commit
│   │   ├── ddd-layer-direction
│   │   │   └── src
│   │   │       └── contexts
│   │   │           └── cart
│   │   │               ├── application
│   │   │               │   └── commands
│   │   │               │       └── add-item
│   │   │               │           └── add-item.handler.ts   you add, commit
│   │   │               └── infrastructure
│   │   │                   └── cart.memory-repository.ts   you add, commit
│   │   └── ddd-src-roots
│   │       └── src
│   │           └── lib
│   │               └── utils.ts   you add, commit
│   └── rules.yaml           you add, commit
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
│   │   ├── shared
│   │   │   └── utils.ts
│   │   └── ui
│   │       └── button.tsx
│   ├── contexts             you add, commit (the business root of lint.ddd.root)
│   │   ├── cart
│   │   │   ├── application
│   │   │   │   └── commands
│   │   │   │       └── add-item
│   │   │   │           ├── add-item.command.ts
│   │   │   │           └── add-item.handler.ts
│   │   │   ├── domain
│   │   │   │   ├── cart-total.entity.ts
│   │   │   │   ├── cart.aggregate.ts
│   │   │   │   ├── cart.repository.ts
│   │   │   │   ├── checkout-date.entity.ts
│   │   │   │   └── legacy-total.entity.ts
│   │   │   ├── infrastructure
│   │   │   │   └── cart.memory-repository.ts
│   │   │   └── cart.module.ts
│   │   ├── catalog
│   │   │   ├── application
│   │   │   │   └── queries
│   │   │   │       └── get-product
│   │   │   │           ├── get-product.handler.ts
│   │   │   │           └── get-product.query.ts
│   │   │   ├── domain
│   │   │   │   ├── product.aggregate.ts
│   │   │   │   └── product.repository.ts
│   │   │   ├── infrastructure
│   │   │   │   └── product.memory-repository.ts
│   │   │   └── catalog.module.ts
│   │   └── kernel.ts
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

64 directories, 69 files
```

The tree comes from the end of the proof. It holds `checkout-date.entity.ts` and `legacy-total.entity.ts` from the `ddd` section, and the `.harness.yaml` there holds `domain_allow: [date-fns]`.
`next-env.d.ts` and `tsconfig.tsbuildinfo` are not harness files. `.gitignore` of the app ignores them.
The harness keeps the state of `harness verify` in `.git/.harness/state.db`, so the tree has no `.harness/` folder. `.harness/` appears when you run `harness run`, and the `.gitignore` line `.harness/` ignores it.
`node_modules/` is a mount point on the host. The packages are in the named volume.
