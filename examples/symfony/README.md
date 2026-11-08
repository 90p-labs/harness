# Symfony example

This example shows a Makefile as the gate source for a Symfony project.
It also shows the architecture rules of the `lint-setup` skill with deptrac, and the canaries that prove them.
Each gate and deptrac run in a container. The harness, the hooks and the plugin scripts run on the host.

## What the folder holds

Only the files that you add to your project:

| File | Purpose |
|---|---|
| `Makefile` | The install target and the gate targets. A `## setup` or `## gate` comment marks each one. |
| `compose.yaml` | The `php` service. It uses the pinned `composer` image. |
| `.harness.yaml` | The `make-gates` and `deptrac` plugins, the lint pack and `verify.parallel: 3`. It has no `setup` key and no `gates` key. |
| `.harness/plugins/make-gates.sh` | The gates plugin. It reads the `## setup` and `## gate` marks. |
| `deptrac.yaml` | The deptrac config: the layers and the allowed dependencies. You own this file. |
| `lint/architecture/pack.yaml` | The custom lint pack `architecture`. Its two rules name the engine `deptrac`. |
| `.harness/plugins/deptrac.sh` | The lint engine plugin. It answers `lint.check`, runs deptrac and maps each violation to a rule id of the catalogue. |
| `lint/rules.yaml` | The manifest. It gives a canary or a waiver for each rule of the `ddd` catalogue pack. |
| `lint/canaries/` | One folder for each rule with a canary. Each folder breaks its rule on purpose. |

## Prerequisites

1. Install Docker and Compose. The proof used Docker Engine 29.9.2 and Compose v5.6.1.
2. Put `harness`, `make`, `sh` and `jq` on the `PATH` of the host. Each gate runs `make <target>` on the host. The deptrac plugin runs on the host and uses `jq`.
3. Use a Symfony project with `phpunit/phpunit`, `phpstan/phpstan`, `friendsofphp/php-cs-fixer` and `deptrac/deptrac` in `require-dev`.

The proof used `symfony/skeleton` v7.4.99, `phpunit/phpunit` 13.4.6, `phpstan/phpstan` 2.3.5, `friendsofphp/php-cs-fixer` v3.95.29, `deptrac/deptrac` 4.7.2 and jq 1.6.
The image is `composer:2.11.1`. It holds PHP 8.5.12, git and unzip.

## How it works

1. Mark the install target with `## setup creates: vendor/autoload.php` on the target line.
2. Mark each gate target with `## gate` on the target line.
3. Mark the test target with `## gate needs: cs phpstan lint-container`.
4. `harness gates init` sends the `gates.detect` event to the plugin.
5. The plugin reads `./Makefile` and prints one `allow` verdict. Its `context` holds one line for each marked target.
6. The harness writes the `install` line under the `setup` key and the gate lines under the `gates` key of `.harness.yaml`. The `lint` key gives the `lint` gate. `lint/rules.yaml` gives the `lint-canaries` gate.
7. `harness verify` runs the setup step, then the gates. Each target calls `docker compose run --rm -T php`.

The `## setup` marker makes the `install` target a setup step with the id `make-install`.
The step runs `composer install`. It runs again only when the hash of `composer.json`, `composer.lock`, `compose.yaml` or `Makefile` changes, or when `vendor/autoload.php` is missing.
A step that runs prints `setup make-install <duration>` before the gate lines. A step that does not run prints nothing.
`vendor/` is in the `.gitignore` of Symfony. The temporary worktree of `harness verify --commits` has no `vendor/`, so the step runs there.
If the step fails, `harness verify` stops before the first gate and prints `setup failed: make-install`.

The `needs` marker gives the `needs` key of the gate. The `make-test` gate starts only after `make-cs`, `make-phpstan` and `make-lint-container` are `PASS` or `CACHED`.
If a gate in the list fails, the harness prints `SKIPPED make-test needs <gate>` and `harness verify` exits with 1.
The plugin exits with 1 on an unknown marker.
`verify.parallel: 3` in `.harness.yaml` sets the most gates that run at the same time. The default is 1. The gates without `needs` run three at a time.

Set the top-level `name:` of `compose.yaml` to the name of your project. The fixed name lets the temporary worktree of `harness verify --commits` use the same composer cache volume.
The plugin runs only when `harness gates init` or `harness init` writes the `gates` key.
To add a gate, mark the new target. Then edit the `gates` key by hand, or remove it and run `harness gates init` again.

## Steps

Copy the files into your project. Then add these lines to `.gitignore`:

```
.harness/*
!.harness/plugins/
```

The first line ignores the run state of the harness. The second line keeps the plugins in git.
Commit the plugins. The command `git ls-files .harness` must list them:

```console
$ git ls-files .harness
.harness/plugins/deptrac.sh
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

Write the setup step and the gates, then the lint files:

```console
$ harness gates init
setup make-install: make install
lint: harness lint --json
lint-canaries: harness lint canaries
make-cs: make cs
make-phpstan: make phpstan
make-lint-container: make lint-container
make-test: make test
$ harness lint init
$ harness lint baseline
```

`harness lint init` writes `.harness-lint/VERSION`. `harness lint baseline` writes `.harness-baseline/lint.json` with no findings.
The harness writes one setup step under the `setup` key and six gates under the `gates` key of `.harness.yaml`. The listing shows the setup step and the first three gates:

```yaml
plugins:
  - name: make-gates
    command: [sh, .harness/plugins/make-gates.sh]
    events: [gates.detect]
  - name: deptrac
    command: [sh, -c, 'exec sh "$(git rev-parse --show-toplevel)/.harness/plugins/deptrac.sh"']
    events: [lint.check]
    timeout: 60s
lint:
  packs: [./lint/architecture]
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
  - id: lint
    run: harness lint --json
    baseline: .harness-baseline/lint.json
  - id: lint-canaries
    run: harness lint canaries
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

The four `make` gates have the same `paths`. The setup step has its own `paths`. The `lint` and `lint-canaries` gates have no `paths`, so every tracked file is their input.
Commit `.harness.yaml`, `.harness-lint/` and `.harness-baseline/`. Then run the gates two times:

```console
$ harness verify
setup make-install 7.288s
PASS lint 2.44s
PASS lint-canaries 4.674s
PASS make-cs 1.073s
PASS make-phpstan 4.836s
PASS make-lint-container 953ms
PASS make-test 851ms
$ harness verify
CACHED lint 2.44s
CACHED lint-canaries 4.674s
CACHED make-cs 1.072s
CACHED make-phpstan 4.835s
CACHED make-lint-container 953ms
CACHED make-test 850ms
```

The first run prints the setup line, then `PASS` for each gate. Three gates without `needs` run at the same time. `make-test` runs after `make-cs`, `make-phpstan` and `make-lint-container`.
The second run prints no setup line and `CACHED` for each gate, because no input changed.

## A failed gate

The proof moved the opening brace of `class Kernel` in `src/Kernel.php` to the line of the class. php-cs-fixer reports that change. The gates without `needs` ran. `make-cs` failed, and the harness skipped `make-test`:

```console
$ harness verify
PASS lint 2.342s
PASS lint-canaries 4.888s
FAIL make-cs 1.023s
PASS make-phpstan 5.156s
PASS make-lint-container 1.665s
SKIPPED make-test needs make-cs

make-cs:
docker compose run --rm -T php vendor/bin/php-cs-fixer fix --dry-run --diff
 Container my-symfony-app-php-run-3355e7f2b058 Creating 
 Container my-symfony-app-php-run-3355e7f2b058 Created 
You are running PHP CS Fixer on PHP 8.5.12, but the minimum PHP version supported by your project in composer.json is PHP 8.2. Executing PHP CS Fixer on newer PHP versions may introduce syntax or features not yet available in PHP 8.2, which could cause issues under that version. It is recommended to run PHP CS Fixer on PHP 8.2, to fit your project specifics.
If you need help while solving warnings, ask at https://github.com/PHP-CS-Fixer/PHP-CS-Fixer/discussions/, we will help you!

PHP CS Fixer 3.95.29 Adalbertus by Fabien Potencier, Dariusz Ruminski and contributors.
PHP runtime: 8.5.12
Loaded config default from "/app/.php-cs-fixer.dist.php".
Running analysis on 9 cores with 10 files per process.
Using cache file ".php-cs-fixer.cache".
 0/8 [░░░░░░░░░░░░░░░░░░░░░░░░░░░░]   0%
 1/8 [▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░░]  12%
 8/8 [▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓] 100%

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


Found 1 of 8 files that can be fixed in 0.026 seconds, 20.00 MB memory used

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
setup make-install 7.294s
CACHED lint 2.44s
CACHED lint-canaries 4.674s
CACHED make-cs 1.072s
CACHED make-phpstan 4.835s
CACHED make-lint-container 953ms
CACHED make-test 850ms
```

A change of `composer.lock` also runs the step, because `composer.lock` is in the `paths` of the step.

`harness verify --commits` checks each commit in a temporary worktree. The proof added one commit that appends `APP_NAME=shop` to `.env`, so every gate ran again there. The worktree has no `vendor/`, so the setup step ran there too. The output does not print the setup line:

```console
$ harness verify --commits HEAD~1
PASS 54e2e60eba3b chore
```

## Architecture rules with deptrac

The `lint-setup` skill wrote these files with its steps. `harness lint catalogue` prints the rules of the catalogue.
The manifest `lint/rules.yaml` names the catalogue pack `ddd` and gives each of its four rules a canary or a waiver:

```yaml
pack: ddd
rules:
  - id: ddd/domain-purity
    canary: lint/canaries/ddd-domain-purity
  - id: ddd/layer-direction
    canary: lint/canaries/ddd-layer-direction
  - id: ddd/cross-context
    waived: deptrac needs one layer for each bounded context, and the project has one context. Add the layers and a canary with the second context.
  - id: ddd/file-suffix
    waived: deptrac checks the dependencies, not the file names.
```

The project does not turn on the built-in `ddd` pack, because its engine reads TypeScript.
The custom pack `architecture` gives the two rules with a canary to the engine `deptrac`. Its scope `src/**/*.php` leaves out `lint/canaries/`.
`deptrac.yaml` reads only `./src`, so a canary never fails the `lint` gate.

`deptrac.yaml` puts each class in one layer by its namespace: `App\<Context>\Domain`, `App\<Context>\Application` and `App\<Context>\Infrastructure`. The `Framework` layer holds `Symfony` and `Doctrine`.
The `ruleset` lets the domain use nothing, the application layer use the domain, and the infrastructure layer use all three.
Edit the layers and the ruleset to match your project. Then run `harness lint canaries` to see that each rule still catches its canary.

The plugin `deptrac.sh` does these steps:

1. It reads the event from stdin before it starts the container.
2. It reads the folder of the call with `git rev-parse --show-prefix`. The harness runs the plugin in the canary folder for a canary, and in the project root for `harness lint`.
3. For a canary, it writes a deptrac config that imports `deptrac.yaml` and sets `projectDirectory` to the canary folder. deptrac resolves `./src` from that folder.
4. It runs deptrac with the JSON formatter in the container. An invalid config has no report, so the plugin prints the deptrac text to stderr and exits with 1.
5. It gives the event and the report to `jq` on stdin. A violation of the `Domain` layer is a `ddd/domain-purity` finding. A violation of the `Application` layer is a `ddd/layer-direction` finding.
6. It keeps only the files and the rules of the event.

A domain class that uses a framework class gives a finding. The proof added `src/Order/Domain/Order.php` with a constructor that takes a `Request`:

```console
$ harness lint
error ddd/domain-purity src/Order/Domain/Order.php:9 App\Order\Domain\Order must not depend on Symfony\Component\HttpFoundation\Request (Domain on Framework)
error: lint found error findings
$ echo $?
1
```

### The canaries pass

`harness lint canaries` sends `lint.check` to the engine of each rule with a canary, in the canary folder, with only that rule. The rule must be in the findings. No agent runs in this check:

```console
$ harness lint canaries
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
PASS ddd/layer-direction lint/canaries/ddd-layer-direction
WAIVED ddd/cross-context: deptrac needs one layer for each bounded context, and the project has one context. Add the layers and a canary with the second context.
WAIVED ddd/file-suffix: deptrac checks the dependencies, not the file names.
$ echo $?
0
```

### A config edit breaks a rule

The proof let the application layer use the infrastructure layer:

```diff
     Application:
       - Domain
+      - Infrastructure
```

deptrac then gave no finding for the canary `ddd-layer-direction`, and the check failed:

```console
$ harness lint canaries
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
FAIL ddd/layer-direction lint/canaries/ddd-layer-direction: the engine deptrac gave no ddd/layer-direction finding
WAIVED ddd/cross-context: deptrac needs one layer for each bounded context, and the project has one context. Add the layers and a canary with the second context.
WAIVED ddd/file-suffix: deptrac checks the dependencies, not the file names.
error: a lint canary failed
$ echo $?
1
```

The `lint` gate stays green after this edit, because the code has no violation. The `lint-canaries` gate fails, so `harness verify` fails.

### A waiver accepts the change

When the team wants the edit, it waives the rule in `lint/rules.yaml` with a reason:

```diff
   - id: ddd/layer-direction
-    canary: lint/canaries/ddd-layer-direction
+    waived: the team lets the application layer use the infrastructure layer.
```

```console
$ harness lint canaries
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
WAIVED ddd/layer-direction: the team lets the application layer use the infrastructure layer.
WAIVED ddd/cross-context: deptrac needs one layer for each bounded context, and the project has one context. Add the layers and a canary with the second context.
WAIVED ddd/file-suffix: deptrac checks the dependencies, not the file names.
$ echo $?
0
```

The output shows each waiver with its reason, so a review sees it. The proof then restored `deptrac.yaml` and `lint/rules.yaml`.

## The project after the setup

The tree comes from `tree -a -I "vendor|node_modules|.git|.venv|var" --dirsfirst` in the project of the proof.
The marks on the right are not part of the `tree` output. "You add" is a file that you copy or edit. "Harness writes" names the command that writes the file.

```text
.
├── .harness
│   └── plugins
│       ├── deptrac.sh           you add, commit
│       └── make-gates.sh        you add, commit
├── .harness-baseline
│   └── lint.json                harness writes (lint baseline), commit
├── .harness-lint
│   └── VERSION                  harness writes (lint init), commit
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
├── lint
│   ├── architecture
│   │   └── pack.yaml            you add, commit
│   ├── canaries
│   │   ├── ddd-domain-purity
│   │   │   └── src
│   │   │       └── Order
│   │   │           └── Domain
│   │   │               └── Order.php                you add, commit
│   │   └── ddd-layer-direction
│   │       └── src
│   │           └── Order
│   │               ├── Application
│   │               │   └── PlaceOrder.php           you add, commit
│   │               └── Infrastructure
│   │                   └── OrderTable.php           you add, commit
│   └── rules.yaml               you add, commit
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
├── .harness.yaml            you add the plugins and the pack, harness writes setup and gates (gates init), commit
├── .php-cs-fixer.cache
├── .php-cs-fixer.dist.php
├── Makefile                 you add, commit
├── compose.yaml             you add, commit
├── composer.json
├── composer.lock
├── deptrac.yaml             you add, commit
├── phpunit.dist.xml
└── symfony.lock

25 directories, 41 files
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
