# Examples

Each folder holds only the files that you add to a project to set up the harness. It is not a full app. Each example runs its gates in a Docker Compose toolchain service with a pinned image. The harness, the hooks and the guard run on the host.

| Example | Project | Gate source | Toolchain service |
|---|---|---|---|
| [`symfony/`](symfony/) | Symfony | A Makefile with `## gate` targets and a `gates.detect` plugin | `php`, the `composer` image |
| [`go/`](go/) | Go module | The `gates` key, with the ids of the built-in detection of `go.mod` | `go`, the `golang` image |
| [`nextjs/`](nextjs/) | Next.js and shadcn | The `gates` key and the lint packs `nextjs` and `shadcn` | `node`, a `Dockerfile` with a pinned pnpm |

The output in each README is real, from a scratch project that the steps built from zero.

## Rules for every example

- Pin each image to an exact version tag and its index digest, such as `golang:1.27.3-bookworm@sha256:...`. Then every user gets the same toolchain on `linux/arm64` and `linux/amd64`.
- Commit the lockfile of the project (`composer.lock`, `go.sum` or `pnpm-lock.yaml`) and put it in the gate `paths`.
- Put `compose.yaml`, and the `Dockerfile` if you have one, in the gate `paths`. The image is not in the input hash, so an image change must change a file in the hash.
- Pull or build the image before the first `harness verify`. A pull inside a gate counts against the gate timeout of 10 minutes, and a failed gate is not cached.
- Set the top-level `name:` of `compose.yaml` to the name of your project. `harness verify --commits` runs the gates in a temporary worktree, and a fixed name lets that worktree use the same volumes and image. Two projects on one machine need two names.
- Set `COMPOSE_PROJECT_NAME` to a different value in each checkout that runs gates at the same time, such as a second git worktree. The variable overrides `name:`. Two checkouts with one name share the volumes, and two installs into one volume can break each other. Export the variable in the shell that starts the harness. A `.env` file that git does not track is not in the temporary worktree of `harness verify --commits`.
- Do not start two `harness verify` runs at the same time in one checkout. They share one `node_modules` volume or one `vendor/`, and the harness has no lock between two processes.
- Install the dependencies in each gate that needs them, such as `pnpm install --frozen-lockfile` or `composer install`. A lockfile change runs the gate again. The temporary worktree of `harness verify --commits` has no `vendor/`, and it shares the `node_modules` volume with your checkout.
- The output in these READMEs comes from macOS with Docker Desktop, which gives the files that a container writes to the host user. On Linux with Docker Engine, a container runs as root, and the files that it writes in the project belong to root. Then `harness verify --commits` can fail to remove its temporary worktree. Run Docker Engine in rootless mode, which maps root in the container to your user. A `user:` key alone does not work: the named volumes and the caches under `/root` belong to root.
- Add `.harness/` to `.gitignore`, because it holds the run state. A project with a plugin under `.harness/plugins/` uses `.harness/*` and `!.harness/plugins/` instead.

## A Makefile as a gate source

Many projects already wrap each check in a Makefile target. A small exec plugin turns those targets into gates. The [`symfony/`](symfony/) example uses this pattern.

1. Mark each Makefile target that is a gate with a `## gate` comment on its target line:

   ```make
   test: install ## gate
   	docker compose run --rm -T php vendor/bin/phpunit
   ```

2. Add an exec plugin on `gates.detect`. It reads the Makefile and prints one gate line for each marked target, `{"id":"make-<target>","run":"make <target>","paths":[...]}`, inside the `context` of an allow verdict. The plugin of the Symfony example is [`symfony/.harness/plugins/make-gates.sh`](symfony/.harness/plugins/make-gates.sh). The [Plugins](../README.md#plugins) section of the root README gives the verdict format.

3. Register the plugin under `plugins:` in `.harness.yaml`:

   ```yaml
   plugins:
     - name: make-gates
       command: [sh, .harness/plugins/make-gates.sh]
       events: [gates.detect]
   ```

4. Run `harness gates init`. It writes one gate for each marked target. Then `harness verify` runs them, and a second run with no change prints `CACHED`:

   ```console
   $ harness gates init
   make-test: make test
   make-phpstan: make phpstan
   make-cs: make cs
   make-lint-container: make lint-container
   $ harness verify
   PASS make-test 4.344s
   PASS make-phpstan 9.099s
   PASS make-cs 3.605s
   PASS make-lint-container 3.518s
   $ harness verify
   CACHED make-test 4.343s
   CACHED make-phpstan 9.098s
   CACHED make-cs 3.605s
   CACHED make-lint-container 3.518s
   ```

Know these limits:

- The plugin runs only when `harness gates init` or `harness init` writes the `gates` key. A new marked target needs a new `harness gates init`, after you remove the `gates` key, or a hand edit of the `gates` key.
- Set `paths` on each gate: the sources, the tests, the lockfile, the Makefile and `compose.yaml`. A gate with no `paths` hashes the full tree, so any change runs it again.
- The files in `.gitignore`, such as `__pycache__/`, `vendor/` and `node_modules/`, are not in the input hash.
