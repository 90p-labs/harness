# Examples

Each folder holds only the files that you add to a project to set up the harness. It is not a full app. Each example runs its gates in a Docker Compose toolchain service with a pinned image. The harness, the hooks and the guard run on the host.

| Example | Project | Gate source | Toolchain service |
|---|---|---|---|
| [`symfony/`](symfony/) | Symfony | A Makefile with `## setup` and `## gate` targets and a `gates.detect` plugin | `php`, the `composer` image |
| [`go/`](go/) | Go module | The `setup` and `gates` keys, with the gate ids of the built-in detection of `go.mod` | `go`, the `golang` image |
| [`nextjs/`](nextjs/) | Next.js and shadcn | The `setup` and `gates` keys and the lint packs `nextjs` and `shadcn` | `node`, a `Dockerfile` with a pinned pnpm |
| [`python/`](python/) | Python with uv | The `setup` and `gates` keys, written by hand, and custom lint packs with ruff and import-linter lint engine plugins | `python`, the `uv` image |

The output in each README is real, from a scratch project that the steps built from zero.

## Architecture rules

Each example also holds the architecture rules that the `lint-setup` skill writes for its language. `harness lint catalogue` prints the rules.

| Example | Language | Tool | Where the rules live | Canaries |
|---|---|---|---|---|
| [`symfony/`](symfony/README.md#architecture-rules-with-deptrac) | PHP | deptrac | `deptrac.yaml`, the plugin `.harness/plugins/deptrac.sh`, the pack `lint/architecture/` and `lint/rules.yaml` | 2, and 2 waivers |
| [`go/`](go/README.md#architecture-rules-with-go-arch-lint) | Go | go-arch-lint | `.go-arch-lint.yml`, the plugin `.harness/plugins/go-arch-lint.sh`, the pack `lint/architecture/` and `lint/rules.yaml` | 3, and 1 waiver |
| [`python/`](python/README.md#architecture-rules-with-import-linter) | Python | import-linter, with the custom contract type `domain_purity` | `.importlinter`, the plugins `.harness/plugins/import-linter.sh` and `domain_purity.py`, the pack `lint/architecture/` and `lint/rules.yaml` | 4, and 1 waiver |
| [`nextjs/`](nextjs/README.md#architecture-rules-with-the-built-in-ddd-pack) | TypeScript | the built-in `ddd` pack | `ddd` in `lint.packs`, the `lint.ddd` keys of `.harness.yaml` and the optional `lint/rules.yaml` | 5, optional: the tests of the binary also prove the built-in rules |

The gate `lint-canaries` runs `harness lint canaries`. It proves that each rule still catches its canary, with no agent. A config edit that lets a rule through fails the gate.
A waiver in `lint/rules.yaml` accepts the change. `harness lint canaries` prints its reason, but `harness verify` prints only the gate result, so a review reads the waiver in the diff.
A canary proves that its rule catches one fixture. It does not prove that the rule catches each violation in the project.

## Rules for every example

- Pin each image to an exact version tag and its index digest, such as `golang:1.27.3-bookworm@sha256:...`. Then every user gets the same toolchain on `linux/arm64` and `linux/amd64`.
- Commit the lockfile of the project (`composer.lock`, `go.sum`, `pnpm-lock.yaml` or `uv.lock`) and put it in the gate and setup `paths`.
- Put `compose.yaml`, and the `Dockerfile` if you have one, in the gate and setup `paths`. The image is not in the input hash, so an image change must change a file in the hash.
- Pull or build the image before the first `harness verify`. A pull inside a gate counts against the gate timeout of 10 minutes, and a failed gate is not cached.
- Set the top-level `name:` of `compose.yaml` to the name of your project. `harness verify --commits` runs the gates in a temporary worktree, and a fixed name lets that worktree use the same volumes and image. Two projects on one machine need two names.
- Set `COMPOSE_PROJECT_NAME` to a different value in each checkout that runs gates at the same time, such as a second git worktree. The variable overrides `name:`. Two checkouts with one name share the volumes, and two installs into one volume can break each other. Export the variable in the shell that starts the harness. A `.env` file that git does not track is not in the temporary worktree of `harness verify --commits`.
- The harness keeps one setup pass for each step of the repo, and all checkouts of the repo share it. A checkout with its own `COMPOSE_PROJECT_NAME` starts with empty volumes, but its lockfile hash can match the last pass of another checkout. Then `harness verify` does not run the setup step, and the container gates fail. Run the `run` command of the setup step by hand once in that checkout. Do the same after you empty a volume by hand.
- Do not start two `harness verify` runs at the same time in one checkout. They share one `node_modules` volume or one `vendor/`, and the harness has no lock between two processes.
- Put the dependency install in one `setup` step, such as `pnpm install --frozen-lockfile` or `composer install`, and not in a gate. `harness verify` runs the step before the gates, so two gates in parallel never write `vendor/` or `node_modules/` at the same time. Put the lockfile, `compose.yaml` and the `Dockerfile` in the step `paths`: a change runs the install again. Set `creates` to a file that the install writes in the project, such as `vendor/autoload.php`. The temporary worktree of `harness verify --commits` has no such file, so the step runs there too.
- Set `verify.parallel` to 2 or more, and let the fast checks run in parallel. Give the slow gates, such as the tests or the build, a `needs` on the fast checks. When a fast check fails, the slow gates print `SKIPPED` and do not start.
- The output in these READMEs comes from macOS with Docker Desktop, which gives the files that a container writes to the host user. On Linux with Docker Engine, a container runs as root, and the files that it writes in the project belong to root. Then `harness verify --commits` can fail to remove its temporary worktree. Run Docker Engine in rootless mode, which maps root in the container to your user. A `user:` key alone does not work: the named volumes and the caches under `/root` belong to root.
- Add `.harness/` to `.gitignore`, because it holds the run state. A project with a plugin under `.harness/plugins/` uses `.harness/*` and `!.harness/plugins/` instead.

## A Makefile as a gate source

Many projects already wrap each check in a Makefile target. A small exec plugin turns those targets into gates. The [`symfony/`](symfony/) example uses this pattern.

1. Mark each Makefile target that is a gate with a `## gate` comment on its target line. Add `needs:` and the targets that must pass first. Mark the install target with `## setup`, and add `creates:` and the file that it writes:

   ```make
   install: ## setup creates: vendor/autoload.php
   	docker compose run --rm -T php composer install --no-interaction --no-progress

   cs: ## gate
   	docker compose run --rm -T php vendor/bin/php-cs-fixer fix --dry-run --diff

   test: ## gate needs: cs phpstan lint-container
   	docker compose run --rm -T php vendor/bin/phpunit
   ```

2. Add an exec plugin on `gates.detect`. It reads the Makefile and prints one line for each marked target, `{"id":"make-<target>","run":"make <target>","paths":[...]}`, inside the `context` of an allow verdict. A gate line can add `"needs":["make-<target>"]`. A setup line adds `"setup":true` and `"creates":"<file>"`. The plugin exits 1 on an unknown marker. The plugin of the Symfony example is [`symfony/.harness/plugins/make-gates.sh`](symfony/.harness/plugins/make-gates.sh). The [Plugins](../README.md#plugins) section of the root README gives the verdict format.

3. Register the plugin under `plugins:` in `.harness.yaml`:

   ```yaml
   plugins:
     - name: make-gates
       command: [sh, .harness/plugins/make-gates.sh]
       events: [gates.detect]
   ```

4. Run `harness gates init`. It writes the setup step and one gate for each marked target. Then `harness verify` runs the setup step and the gates. A second run with no change prints no setup line and `CACHED` for each gate:

   ```console
   $ harness gates init
   setup make-install: make install
   make-cs: make cs
   make-phpstan: make phpstan
   make-lint-container: make lint-container
   make-test: make test
   $ harness verify
   setup make-install 2.755s
   PASS make-cs 1.287s
   PASS make-phpstan 3.858s
   PASS make-lint-container 1.083s
   PASS make-test 814ms
   $ harness verify
   CACHED make-cs 1.286s
   CACHED make-phpstan 3.857s
   CACHED make-lint-container 1.082s
   CACHED make-test 814ms
   ```

Know these limits:

- The plugin runs only when `harness gates init` or `harness init` writes the `setup` and `gates` keys. A new marked target needs a new `harness gates init`, after you remove the two keys, or a hand edit of the keys.
- Set `paths` on each gate: the sources, the tests, the lockfile, the Makefile and `compose.yaml`. A gate with no `paths` hashes the full tree, so any change runs it again.
- The files in `.gitignore`, such as `__pycache__/`, `vendor/` and `node_modules/`, are not in the input hash.

## Your own lint pack

A team linter can feed the same lint report, baseline ratchet and gate `lint` as the built-in packs. The [`python/`](python/) example is the worked example: a ruff pack and an import-linter pack for a Python project.

1. Write a pack folder, such as `lint/python/pack.yaml`. Each rule names a lint engine plugin as its `engine`, and its `scope` selects the files.
2. Write an exec plugin on `lint.check`. It reads the rules and the files from the payload, runs the linter, and returns `{"verdict":"allow","findings":[...]}`. Each finding needs a rule id of the payload and a file that is relative to the repository.
3. Register the plugin under `plugins:`, and add the pack folder to `lint.packs`, such as `./lint/python`. Set `timeout` above the start time of the linter. The default is 5 s.
4. Run `harness lint init`, `harness lint` and `harness lint baseline`. Then add the gate `lint`, `harness lint --json` with the baseline `.harness-baseline/lint.json`.

The harness calls a lint engine only when an active pack has a rule for it. A pack with only plugin rules needs no Biome, no oxlint and no Node.
Run `harness lint init` again after each edit of the pack. The [Custom lint pack](../README.md#custom-lint-pack-and-lint-engine-plugin) section of the root README gives the contract.
