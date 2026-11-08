# Symfony example

This example shows a Makefile as the gate source for a Symfony project.
Each gate runs in a container. The harness and the hooks run on the host.

## What the folder holds

Only the files that you add to your project:

| File | Purpose |
|---|---|
| `Makefile` | The gate targets. A `## gate` comment marks each gate. |
| `compose.yaml` | The `php` service. It uses the pinned `composer` image. |
| `.harness.yaml` | The `make-gates` plugin. It has no `gates` key. |
| `.harness/plugins/make-gates.sh` | The plugin. It reads the `## gate` marks. |

## Prerequisites

1. Install Docker and Compose. The proof used Docker 29.9.2 and Compose v5.6.1.
2. Put `harness` and `make` on the `PATH` of the host. Each gate runs `make <target>` on the host.
3. Use a Symfony project with `phpunit/phpunit`, `phpstan/phpstan` and `friendsofphp/php-cs-fixer` in `require-dev`.

The proof used `symfony/skeleton` v7.4.99, `phpunit/phpunit` 13.4.6, `phpstan/phpstan` 2.3.5 and `friendsofphp/php-cs-fixer` v3.95.29.
The image is `composer:2.11.1`. It holds PHP 8.5.12, git and unzip.

## How it works

1. Mark each gate target in the `Makefile` with `## gate` on the target line.
2. `harness gates init` sends the `gates.detect` event to the plugin.
3. The plugin reads `./Makefile` and prints one `allow` verdict. Its `context` holds one gate line for each marked target.
4. The harness writes the gates under the `gates` key of `.harness.yaml`.
5. `harness verify` runs each gate. Each gate calls `docker compose run --rm -T php`.

The `install` target has no mark, so it is not a gate. Each gate target depends on `install`, so each gate runs `composer install` first. A `composer.lock` change then installs the new versions, and the temporary worktree of `harness verify --commits` gets its own `vendor/`.

Set the top-level `name:` of `compose.yaml` to the name of your project. The fixed name lets the temporary worktree of `harness verify --commits` use the same composer cache volume.
The plugin runs only when `harness gates init` or `harness init` writes the `gates` key.
To add a gate, mark the new target. Then edit the `gates` key by hand, or remove it and run `harness gates init` again.

## Steps

Copy the four files into your project. Then add these lines to `.gitignore`:

```
.harness/*
!.harness/plugins/
```

The first line ignores the run state of the harness. The second line keeps the plugin in git.
Commit the plugin. The command `git ls-files .harness` must list it:

```console
$ git ls-files .harness
.harness/plugins/make-gates.sh
```

Pull the image before the first `harness verify`. A pull inside a gate can exceed the gate timeout.
A first pull downloads 77 MB of compressed layers on `linux/arm64` and on `linux/amd64`, as the registry manifest gives them.
The time depends on your network. The image is 307MB on disk.
The pull below took 1.1 s with Docker 29.9.2 and Compose v5.6.1, because the host already held the layers.

```console
$ time docker compose pull
 Image composer:2.11.1@sha256:8de2184c6de224f4689e8f80ed5f5edaf0030b6b8b7839a22a2bcb6722ac87d0 Pulling 
 Image composer:2.11.1@sha256:8de2184c6de224f4689e8f80ed5f5edaf0030b6b8b7839a22a2bcb6722ac87d0 Pulled 

real	0m1.062s
user	0m0.057s
sys	0m0.075s
```

Run the plugin by hand to see the gates:

```console
$ sh .harness/plugins/make-gates.sh </dev/null
{"verdict":"allow","context":"{\"id\":\"make-test\",\"run\":\"make test\",\"paths\":[\"**/*.php\",\"src/**\",\"tests/**\",\"config/**\",\"bin/**\",\"public/**\",\"templates/**\",\"translations/**\",\"composer.json\",\"composer.lock\",\".env*\",\"phpunit.dist.xml\",\"phpunit.xml.dist\",\"phpstan.neon\",\"phpstan.dist.neon\",\"phpstan.neon.dist\",\"phpstan-baseline.neon\",\".php-cs-fixer.dist.php\",\"Makefile\",\"compose.yaml\"]}\n{\"id\":\"make-phpstan\",\"run\":\"make phpstan\",\"paths\":[\"**/*.php\",\"src/**\",\"tests/**\",\"config/**\",\"bin/**\",\"public/**\",\"templates/**\",\"translations/**\",\"composer.json\",\"composer.lock\",\".env*\",\"phpunit.dist.xml\",\"phpunit.xml.dist\",\"phpstan.neon\",\"phpstan.dist.neon\",\"phpstan.neon.dist\",\"phpstan-baseline.neon\",\".php-cs-fixer.dist.php\",\"Makefile\",\"compose.yaml\"]}\n{\"id\":\"make-cs\",\"run\":\"make cs\",\"paths\":[\"**/*.php\",\"src/**\",\"tests/**\",\"config/**\",\"bin/**\",\"public/**\",\"templates/**\",\"translations/**\",\"composer.json\",\"composer.lock\",\".env*\",\"phpunit.dist.xml\",\"phpunit.xml.dist\",\"phpstan.neon\",\"phpstan.dist.neon\",\"phpstan.neon.dist\",\"phpstan-baseline.neon\",\".php-cs-fixer.dist.php\",\"Makefile\",\"compose.yaml\"]}\n{\"id\":\"make-lint-container\",\"run\":\"make lint-container\",\"paths\":[\"**/*.php\",\"src/**\",\"tests/**\",\"config/**\",\"bin/**\",\"public/**\",\"templates/**\",\"translations/**\",\"composer.json\",\"composer.lock\",\".env*\",\"phpunit.dist.xml\",\"phpunit.xml.dist\",\"phpstan.neon\",\"phpstan.dist.neon\",\"phpstan.neon.dist\",\"phpstan-baseline.neon\",\".php-cs-fixer.dist.php\",\"Makefile\",\"compose.yaml\"]}"}
```

Write the gates:

```console
$ harness gates init
make-test: make test
make-phpstan: make phpstan
make-cs: make cs
make-lint-container: make lint-container
```

The harness writes four gates under the `gates` key of `.harness.yaml`. The listing shows the file up to the first gate:

```yaml
plugins:
  - name: make-gates
    command: [sh, .harness/plugins/make-gates.sh]
    events: [gates.detect]
gates:
  - id: make-test
    run: make test
    paths:
      - '**/*.php'
      - src/**
      - tests/**
      - config/**
      - bin/**
      - public/**
      - templates/**
      - translations/**
      - composer.json
      - composer.lock
      - .env*
      - phpunit.dist.xml
      - phpunit.xml.dist
      - phpstan.neon
      - phpstan.dist.neon
      - phpstan.neon.dist
      - phpstan-baseline.neon
      - .php-cs-fixer.dist.php
      - Makefile
      - compose.yaml
```

The four gates have the same `paths`. Commit the file. Then run the gates two times:

```console
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

The first run prints `PASS`. The second run prints `CACHED`, because no input changed.

`harness verify --commits` checks each commit in a temporary worktree. The proof added one commit that appends `APP_NAME=shop` to `.env`, so every gate ran again there:

```console
$ harness verify --commits HEAD~1
PASS 94c977893b6f chore
```

## Gate paths

- Commit `composer.lock`. It is in the gate paths, so a dependency change runs the gates again.
- `compose.yaml` is in the paths. The image is not in the input hash, so an image change must change this file.
- `vendor/` and `var/` are in the `.gitignore` of Symfony. They are not in the hash.
- The `paths` also hold every `.php` file outside `vendor/`, the sources, the tests, `config/`, `bin/`, `translations/`, the `.env` files, the phpunit, phpstan and php-cs-fixer configs, and the `Makefile`. php-cs-fixer checks every `.php` file, so `**/*.php` is in the paths.
- On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
- A file that a gate reads but that its `paths` do not match does not run the gate again. Then `harness verify` prints `CACHED` for a stale result.
- Edit the `PATHS` line of the plugin to match your project.
