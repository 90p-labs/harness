# Symfony example

This example shows a Makefile as the gate source for a Symfony project.
Each gate runs in a container. The harness and the hooks run on the host.

## What the folder holds

Only the files that you add to your project:

| File | Purpose |
|---|---|
| `Makefile` | The install target and the gate targets. A `## setup` or `## gate` comment marks each one. |
| `compose.yaml` | The `php` service. It uses the pinned `composer` image. |
| `.harness.yaml` | The `make-gates` plugin and `verify.parallel: 3`. It has no `setup` key and no `gates` key. |
| `.harness/plugins/make-gates.sh` | The plugin. It reads the `## setup` and `## gate` marks. |

## Prerequisites

1. Install Docker and Compose. The proof used Docker Engine 29.9.2 and Compose v5.6.1.
2. Put `harness` and `make` on the `PATH` of the host. Each gate runs `make <target>` on the host.
3. Use a Symfony project with `phpunit/phpunit`, `phpstan/phpstan` and `friendsofphp/php-cs-fixer` in `require-dev`.

The proof used `symfony/skeleton` v7.4.99, `phpunit/phpunit` 13.4.6, `phpstan/phpstan` 2.3.5 and `friendsofphp/php-cs-fixer` v3.95.29.
The image is `composer:2.11.1`. It holds PHP 8.5.12, git and unzip.

## How it works

1. Mark the install target with `## setup creates: vendor/autoload.php` on the target line.
2. Mark each gate target with `## gate` on the target line.
3. Mark the test target with `## gate needs: cs phpstan lint-container`.
4. `harness gates init` sends the `gates.detect` event to the plugin.
5. The plugin reads `./Makefile` and prints one `allow` verdict. Its `context` holds one line for each marked target.
6. The harness writes the `install` line under the `setup` key and the gate lines under the `gates` key of `.harness.yaml`.
7. `harness verify` runs the setup step, then the gates. Each target calls `docker compose run --rm -T php`.

The `## setup` marker makes the `install` target a setup step with the id `make-install`.
The step runs `composer install`. It runs again only when the hash of `composer.json`, `composer.lock`, `compose.yaml` or `Makefile` changes, or when `vendor/autoload.php` is missing.
A step that runs prints `setup make-install <duration>` before the gate lines. A step that does not run prints nothing.
`vendor/` is in the `.gitignore` of Symfony. The temporary worktree of `harness verify --commits` has no `vendor/`, so the step runs there.
If the step fails, `harness verify` stops before the first gate and prints `setup failed: make-install`.

The `needs` marker gives the `needs` key of the gate. The `make-test` gate starts only after `make-cs`, `make-phpstan` and `make-lint-container` are `PASS` or `CACHED`.
If a gate in the list fails, the harness prints `SKIPPED make-test needs <gate>` and `harness verify` exits with 1.
The plugin exits with 1 on an unknown marker.
`verify.parallel: 3` in `.harness.yaml` sets the most gates that run at the same time. The default is 1. The three gates without `needs` run together.

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
The pull below took 1.2 s with Docker Engine 29.9.2 and Compose v5.6.1, because the host already held the layers.

```console
$ time docker compose pull
 Image composer:2.11.1@sha256:8de2184c6de224f4689e8f80ed5f5edaf0030b6b8b7839a22a2bcb6722ac87d0 Pulling 
 Image composer:2.11.1@sha256:8de2184c6de224f4689e8f80ed5f5edaf0030b6b8b7839a22a2bcb6722ac87d0 Pulled 

real	0m1.223s
user	0m0.057s
sys	0m0.051s
```

Run the plugin by hand to see the gates:

```console
$ sh .harness/plugins/make-gates.sh </dev/null
{"verdict":"allow","context":"{\"id\":\"make-install\",\"run\":\"make install\",\"paths\":[\"composer.json\",\"composer.lock\",\"compose.yaml\",\"Makefile\"],\"setup\":true,\"creates\":\"vendor/autoload.php\"}\n{\"id\":\"make-cs\",\"run\":\"make cs\",\"paths\":[\"**/*.php\",\"src/**\",\"tests/**\",\"config/**\",\"bin/**\",\"public/**\",\"templates/**\",\"translations/**\",\"composer.json\",\"composer.lock\",\".env*\",\"phpunit.dist.xml\",\"phpunit.xml.dist\",\"phpstan.neon\",\"phpstan.dist.neon\",\"phpstan.neon.dist\",\"phpstan-baseline.neon\",\".php-cs-fixer.dist.php\",\"Makefile\",\"compose.yaml\"]}\n{\"id\":\"make-phpstan\",\"run\":\"make phpstan\",\"paths\":[\"**/*.php\",\"src/**\",\"tests/**\",\"config/**\",\"bin/**\",\"public/**\",\"templates/**\",\"translations/**\",\"composer.json\",\"composer.lock\",\".env*\",\"phpunit.dist.xml\",\"phpunit.xml.dist\",\"phpstan.neon\",\"phpstan.dist.neon\",\"phpstan.neon.dist\",\"phpstan-baseline.neon\",\".php-cs-fixer.dist.php\",\"Makefile\",\"compose.yaml\"]}\n{\"id\":\"make-lint-container\",\"run\":\"make lint-container\",\"paths\":[\"**/*.php\",\"src/**\",\"tests/**\",\"config/**\",\"bin/**\",\"public/**\",\"templates/**\",\"translations/**\",\"composer.json\",\"composer.lock\",\".env*\",\"phpunit.dist.xml\",\"phpunit.xml.dist\",\"phpstan.neon\",\"phpstan.dist.neon\",\"phpstan.neon.dist\",\"phpstan-baseline.neon\",\".php-cs-fixer.dist.php\",\"Makefile\",\"compose.yaml\"]}\n{\"id\":\"make-test\",\"run\":\"make test\",\"paths\":[\"**/*.php\",\"src/**\",\"tests/**\",\"config/**\",\"bin/**\",\"public/**\",\"templates/**\",\"translations/**\",\"composer.json\",\"composer.lock\",\".env*\",\"phpunit.dist.xml\",\"phpunit.xml.dist\",\"phpstan.neon\",\"phpstan.dist.neon\",\"phpstan.neon.dist\",\"phpstan-baseline.neon\",\".php-cs-fixer.dist.php\",\"Makefile\",\"compose.yaml\"],\"needs\":[\"make-cs\",\"make-phpstan\",\"make-lint-container\"]}"}
```

Write the setup step and the gates:

```console
$ harness gates init
setup make-install: make install
make-cs: make cs
make-phpstan: make phpstan
make-lint-container: make lint-container
make-test: make test
```

The harness writes one setup step under the `setup` key and four gates under the `gates` key of `.harness.yaml`. The listing shows the setup step and the first gate:

```yaml
plugins:
  - name: make-gates
    command: [sh, .harness/plugins/make-gates.sh]
    events: [gates.detect]
verify:
  parallel: 3
setup:
  - id: make-install
    run: make install
    paths:
      - composer.json
      - composer.lock
      - compose.yaml
      - Makefile
    creates: vendor/autoload.php
gates:
  - id: make-cs
    run: make cs
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

The `make-test` gate ends with the `needs` key:

```yaml
  - id: make-test
    run: make test
    paths:
      # the same paths as above
    needs:
      - make-cs
      - make-phpstan
      - make-lint-container
```

The four gates have the same `paths`. The setup step has its own `paths`. Commit the file. Then run the gates two times:

```console
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

The first run prints the setup line, then `PASS` for each gate. `make-cs`, `make-phpstan` and `make-lint-container` run in parallel. `make-test` runs after them.
The second run prints no setup line and `CACHED` for each gate, because no input changed.

## A failed gate

The proof moved the opening brace of `class Kernel` in `src/Kernel.php` to the line of the class. php-cs-fixer reports that change. The three gates without `needs` ran. `make-cs` failed, and the harness skipped `make-test`:

```console
$ harness verify
FAIL make-cs 2.248s
PASS make-phpstan 12.343s
PASS make-lint-container 2.752s
SKIPPED make-test needs make-cs

make-cs:
docker compose run --rm -T php vendor/bin/php-cs-fixer fix --dry-run --diff
 Container shop-php-run-33726bbbb67e Creating 
 Container shop-php-run-33726bbbb67e Created 
You are running PHP CS Fixer on PHP 8.5.12, but the minimum PHP version supported by your project in composer.json is PHP 8.2. Executing PHP CS Fixer on newer PHP versions may introduce syntax or features not yet available in PHP 8.2, which could cause issues under that version. It is recommended to run PHP CS Fixer on PHP 8.2, to fit your project specifics.
If you need help while solving warnings, ask at https://github.com/PHP-CS-Fixer/PHP-CS-Fixer/discussions/, we will help you!

PHP CS Fixer 3.95.29 Adalbertus by Fabien Potencier, Dariusz Ruminski and contributors.
PHP runtime: 8.5.12
Loaded config default from "/app/.php-cs-fixer.dist.php".
Running analysis on 9 cores with 10 files per process.
Using cache file ".php-cs-fixer.cache".
 0/5 [░░░░░░░░░░░░░░░░░░░░░░░░░░░░]   0%
 1/5 [▓▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░]  20%
 5/5 [▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓] 100%

   1) src/Kernel.php
      ---------- begin diff ----------
--- /app/src/Kernel.php
+++ /app/src/Kernel.php
@@ -5,6 +5,7 @@
 use Symfony\Bundle\FrameworkBundle\Kernel\MicroKernelTrait;
 use Symfony\Component\HttpKernel\Kernel as BaseKernel;
 
-class Kernel extends BaseKernel {
+class Kernel extends BaseKernel
+{
     use MicroKernelTrait;
 }

      ----------- end diff -----------


Found 1 of 5 files that can be fixed in 0.045 seconds, 18.00 MB memory used

make: *** [cs] Error 8

error: gates failed
$ echo $?
1
```

The `echo $?` line is a separate command. It printed the exit code of `harness verify`. The proof then restored `src/Kernel.php`, and the gates were `CACHED` again.

## The setup runs again

The setup step does nothing when no input changed. It runs again when `vendor/autoload.php` is missing. The proof removed `vendor/`. The step ran, and the gates stayed `CACHED`, because `vendor/` is not in the gate paths:

```console
$ rm -rf vendor
$ harness verify
setup make-install 11.85s
CACHED make-cs 1.286s
CACHED make-phpstan 3.857s
CACHED make-lint-container 1.082s
CACHED make-test 814ms
```

A change of `composer.lock` also runs the step, because `composer.lock` is in the `paths` of the step.

`harness verify --commits` checks each commit in a temporary worktree. The proof added one commit that appends `APP_NAME=shop` to `.env`, so every gate ran again there. The worktree has no `vendor/`, so the setup step ran there too. The output does not print the setup line:

```console
$ harness verify --commits HEAD~1
PASS 1b46337c7296 chore
```

## The project after the setup

The tree comes from `tree -a -I "vendor|node_modules|.git|.venv|var" --dirsfirst` in the project of the proof.
The marks on the right are not part of the `tree` output. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.

```text
.
├── .harness
│   └── plugins
│       └── make-gates.sh        you add, commit
├── .phpunit.cache
│   └── test-run-history
├── bin
│   ├── console
│   └── phpunit
├── config
│   ├── packages
│   │   ├── cache.yaml
│   │   ├── framework.yaml
│   │   └── routing.yaml
│   ├── routes
│   │   └── framework.yaml
│   ├── bundles.php
│   ├── preload.php
│   ├── reference.php
│   ├── routes.yaml
│   └── services.yaml
├── public
│   └── index.php
├── src
│   ├── Controller
│   │   └── .gitignore
│   └── Kernel.php
├── tests
│   ├── SmokeTest.php
│   └── bootstrap.php
├── .editorconfig
├── .env
├── .env.dev
├── .env.test
├── .gitignore               you add the harness lines, commit
├── .harness.yaml            you add the plugin, harness writes setup and gates (gates init), commit
├── .php-cs-fixer.cache
├── .php-cs-fixer.dist.php
├── Makefile                 you add, commit
├── compose.yaml             you add, commit
├── composer.json
├── composer.lock
├── phpunit.dist.xml
└── symfony.lock

11 directories, 32 files
```

Git ignores `.phpunit.cache/` and `.php-cs-fixer.cache`. The tree leaves out `vendor/`, `var/` and `.git/`.
The harness writes its run state into `.harness/` when a run starts. `.gitignore` ignores it, except `.harness/plugins/`.

## Gate paths

- Commit `composer.lock`. It is in the gate paths, so a dependency change runs the gates again.
- `compose.yaml` is in the paths. The image is not in the input hash, so an image change must change this file.
- `vendor/` and `var/` are in the `.gitignore` of Symfony. They are not in the hash. The setup step restores `vendor/`.
- The `paths` also hold every `.php` file outside `vendor/`, the sources, the tests, `config/`, `bin/`, `translations/`, the `.env` files, the phpunit, phpstan and php-cs-fixer configs, and the `Makefile`. php-cs-fixer checks every `.php` file, so `**/*.php` is in the paths.
- On Linux with Docker Engine, read the root-user limit in the [examples index](../README.md#rules-for-every-example).
- A file that a gate reads but that its `paths` do not match does not run the gate again. Then `harness verify` prints `CACHED` for a stale result.
- Edit the `PATHS` line of the plugin to match your project.
