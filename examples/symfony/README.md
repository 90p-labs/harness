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
| `deptrac.yaml` | The deptrac config: one layer for each context and layer kind, the library layers and the allowed dependencies. You own this file. |
| `.php-cs-fixer.dist.php` | The php-cs-fixer config of the Symfony recipe, with `lint/canaries` excluded. |
| `lint/architecture/pack.yaml` | The custom lint pack `architecture`. Its four rules name the engine `deptrac`. |
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

Copy the files into your project. The example `.php-cs-fixer.dist.php` replaces the file of the php-cs-fixer recipe. It adds `lint/canaries` to the `exclude` list, because each canary breaks a rule on purpose and php-cs-fixer must not check it. If your file has other changes, add only `lint/canaries` to its `exclude` list.
Then add these lines to `.gitignore`:

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

`harness lint init` writes `.harness-lint/VERSION`. `harness lint baseline` writes `.harness-lint/baseline.json` with no findings, because the code of the proof breaks no rule.
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
    baseline: .harness-lint/baseline.json
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
Commit `.harness.yaml` and `.harness-lint/`. Then run the gates two times:

```console
$ harness verify
setup make-install 3.123s
PASS lint 4.089s
PASS lint-canaries 13.644s
PASS make-cs 1.993s
PASS make-phpstan 8.119s
PASS make-lint-container 1.116s
PASS make-test 2.364s
$ harness verify
CACHED lint 4.088s
CACHED lint-canaries 13.643s
CACHED make-cs 1.993s
CACHED make-phpstan 8.119s
CACHED make-lint-container 1.116s
CACHED make-test 2.364s
```

The first run prints the setup line, then `PASS` for each gate. Three gates without `needs` run at the same time. `make-test` runs after `make-cs`, `make-phpstan` and `make-lint-container`.
The second run prints no setup line and `CACHED` for each gate, because no input changed.

## A failed gate

The proof moved the opening brace of `class Kernel` in `src/Kernel.php` to the line of the class. php-cs-fixer reports that change. The gates without `needs` ran. `make-cs` failed, and the harness skipped `make-test`:

```console
$ harness verify
PASS lint 3.562s
PASS lint-canaries 10.552s
FAIL make-cs 1.683s
PASS make-phpstan 6.52s
PASS make-lint-container 1.392s
SKIPPED make-test needs make-cs

make-cs:
docker compose run --rm -T php vendor/bin/php-cs-fixer fix --dry-run --diff
 Container my-symfony-app-php-run-055114fd59b7 Creating 
 Container my-symfony-app-php-run-055114fd59b7 Created 
You are running PHP CS Fixer on PHP 8.5.12, but the minimum PHP version supported by your project in composer.json is PHP 8.2. Executing PHP CS Fixer on newer PHP versions may introduce syntax or features not yet available in PHP 8.2, which could cause issues under that version. It is recommended to run PHP CS Fixer on PHP 8.2, to fit your project specifics.
If you need help while solving warnings, ask at https://github.com/PHP-CS-Fixer/PHP-CS-Fixer/discussions/, we will help you!

PHP CS Fixer 3.95.29 Adalbertus by Fabien Potencier, Dariusz Ruminski and contributors.
PHP runtime: 8.5.12
Loaded config default from "/app/.php-cs-fixer.dist.php".
Running analysis on 9 cores with 10 files per process.
Using cache file ".php-cs-fixer.cache".
  0/12 [░░░░░░░░░░░░░░░░░░░░░░░░░░░░]   0%
  2/12 [▓▓▓▓░░░░░░░░░░░░░░░░░░░░░░░░]  16%
 12/12 [▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓▓] 100%

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


Found 1 of 12 files that can be fixed in 0.479 seconds, 40.00 MB memory used

make: *** [cs] Error 8

error: gates failed
$ echo $?
1
```

The `echo $?` line is a separate command. It printed the exit code of `harness verify`. The proof then restored `src/Kernel.php`, and the gates were `CACHED` again.
php-cs-fixer checked 12 files. The exclude of `.php-cs-fixer.dist.php` keeps the 6 canary files out:

```console
$ docker compose run --rm -T php vendor/bin/php-cs-fixer list-files 2>/dev/null
'./config/preload.php'
'./tests/PlaceOrderHandlerTest.php'
'./tests/bootstrap.php'
'./public/index.php'
'./src/Order/Application/PlaceOrderHandler.php'
'./src/Order/Infrastructure/InMemoryOrderRepository.php'
'./src/Order/Domain/Order.php'
'./src/Order/Domain/OrderRepository.php'
'./src/Billing/Application/IssueInvoiceHandler.php'
'./src/Billing/Infrastructure/InvoicePrinter.php'
'./src/Billing/Domain/Invoice.php'
'./src/Kernel.php'
```

## The setup runs again

The setup step does nothing when no input changed. It runs again when `vendor/autoload.php` is missing. The proof removed `vendor/`. The step ran, and the gates stayed `CACHED`, because `vendor/` is not in the gate paths:

```console
$ rm -rf vendor
$ harness verify
setup make-install 7.939s
CACHED lint 4.088s
CACHED lint-canaries 13.643s
CACHED make-cs 1.993s
CACHED make-phpstan 8.119s
CACHED make-lint-container 1.116s
CACHED make-test 2.364s
```

A change of `composer.lock` also runs the step, because `composer.lock` is in the `paths` of the step.

`harness verify --commits` checks each commit in a temporary worktree. The proof added one commit that appends `APP_NAME=shop` to `.env`, so every gate ran again there. The worktree has no `vendor/`, so the setup step ran there too. The output does not print the setup line:

```console
$ harness verify --commits HEAD~1
PASS a0e48347076d chore
```

## Architecture rules with deptrac

The `lint-setup` skill wrote these files with its steps. `harness lint catalogue` prints the rules of the catalogue.
The manifest `lint/rules.yaml` names the catalogue pack `ddd` and gives each of its five rules a canary or a waiver:

```yaml
pack: ddd
rules:
  - id: ddd/domain-purity
    canary: lint/canaries/ddd-domain-purity
  - id: ddd/layer-direction
    canary: lint/canaries/ddd-layer-direction
  - id: ddd/cross-context
    canary: lint/canaries/ddd-cross-context
  - id: ddd/file-suffix
    waived: deptrac checks the dependencies, not the file names.
  - id: ddd/src-roots
    canary: lint/canaries/ddd-src-roots
```

The project does not turn on the built-in `ddd` pack, because its engine reads TypeScript.
The custom pack `architecture` gives the four rules with a canary to the engine `deptrac`. Its scope `src/**/*.php` leaves out `lint/canaries/`.
`deptrac.yaml` reads only `./src`, so a canary never fails the `lint` gate.

`deptrac.yaml` has one layer for each context and layer kind. The example has the contexts `Order` and `Billing`, so it has six context layers:

| Layer | Classes | May use |
|---|---|---|
| `OrderDomain`, `BillingDomain` | `App\<Context>\Domain\*` | `SharedDomain`, `DomainLibraries` |
| `OrderApplication`, `BillingApplication` | `App\<Context>\Application\*` | the domain of its context, `SharedDomain`, `DomainLibraries`, `Vendor` |
| `OrderInfrastructure`, `BillingInfrastructure` | `App\<Context>\Infrastructure\*` | the domain and the application layer of its context, `SharedDomain`, `DomainLibraries`, `Vendor`, `Framework` |
| `SharedDomain` | `App\Shared\Domain\*` | `DomainLibraries` |
| `Kernel` | `App\Kernel` | `Framework` |
| `DomainLibraries` | the libraries that the domain may use | nothing |
| `Framework` | `Symfony\*` and `Doctrine\*` | nothing |
| `Vendor` | each other class outside `App` and outside `src/`, except the PHP built-in classes | nothing |

The plugin reads the context and the kind from the layer name: a name that ends in `Domain`, `Application` or `Infrastructure` is a context layer. Keep this form for each new layer, with no space or parenthesis in the name. A deptrac message with no layer name stops the plugin with an error.
`DomainLibraries` holds `^Ramsey\\Uuid\\.*` as a placeholder. Replace it with the list of libraries that your domain may use. deptrac refuses a layer with no collector, so keep one line.
A controller belongs in the infrastructure layer of its context, for example `App\Order\Infrastructure\Http\OrderController`. The example has no layer for `App\Controller`, so a class there is a `ddd/src-roots` finding.
Edit the layers and the ruleset to match your project. Then run `harness lint canaries` to see that each rule still catches its canary.

To add a context, for example `Catalog`, add three layers and three ruleset entries to `deptrac.yaml`:

```yaml
  layers:
    - name: CatalogDomain
      collectors:
        - type: classLike
          value: ^App\\Catalog\\Domain\\.*
    - name: CatalogApplication
      collectors:
        - type: classLike
          value: ^App\\Catalog\\Application\\.*
    - name: CatalogInfrastructure
      collectors:
        - type: classLike
          value: ^App\\Catalog\\Infrastructure\\.*
  ruleset:
    CatalogDomain:
      - SharedDomain
      - DomainLibraries
    CatalogApplication:
      - CatalogDomain
      - SharedDomain
      - DomainLibraries
      - Vendor
    CatalogInfrastructure:
      - CatalogDomain
      - CatalogApplication
      - SharedDomain
      - DomainLibraries
      - Vendor
      - Framework
```

The canary `ddd-src-roots` holds the context `Shipping`, which the config does not list. If your project gets a `Shipping` context, give the canary another context name.

The plugin `deptrac.sh` does these steps:

1. It reads the event from stdin before it starts the container.
2. It reads the folder of the call with `git rev-parse --show-prefix`. The harness runs the plugin in the canary folder for a canary, and in the project root for `harness lint`.
3. For a canary, it writes a deptrac config that imports `deptrac.yaml` and sets `projectDirectory` to the canary folder. deptrac resolves `./src` from that folder.
4. It runs `deptrac analyse --report-uncovered` with the JSON formatter in a container with the name `deptrac-lint-<pid>`. An invalid config has no report, so the plugin prints the deptrac text to stderr and exits with 1.
5. It runs `deptrac debug:unassigned` with the same config. This command lists each class of `src` that is in no layer, and exits with 2 when the list is not empty.
6. It gives the event, the report and the list to `jq`. The rules come from the layer names in each message:
   - A violation from a context layer to a layer of another context is a `ddd/cross-context` finding.
   - Another violation of a `Domain` layer is a `ddd/domain-purity` finding. A violation of an `Application` layer is a `ddd/layer-direction` finding.
   - Any other violation has no catalogue rule, for example an `Infrastructure` layer on `Kernel`. The plugin stops with an error that names the violation.
   - An uncovered dependency is a dependency on a class in no layer. It is a `ddd/src-roots` finding at the file of the dependant class.
   - A class in the unassigned list is a `ddd/src-roots` finding at line 1 of its file. The plugin maps the class `App\X\Y` to the file `src/X/Y.php`, the PSR-4 map of the Symfony skeleton. It maps a class `X\Y` outside `App` to the same file `src/X/Y.php`. A class in another file gives no finding.
7. It keeps only the files and the rules of the event.
8. A plugin timeout sends SIGTERM to the plugin. The signal stops the `docker compose` client, but not the container. The plugin traps SIGTERM and runs `docker kill` with the container name. The `--rm` flag then removes the container.

The proof set the plugin `timeout` to `1s`. No lint container stayed on the host:

```console
$ harness lint

error: lint engine deptrac: plugin failed: deptrac: context deadline exceeded: exit status 143
$ echo $?
1
$ docker ps -a --filter name=-lint- --format '{{.Names}}'
```

The proof added five classes, one for each rule with a canary:

- `src/Order/Domain/OrderSource.php` takes a Symfony `Request`.
- `src/Order/Domain/OrderJournal.php` takes a `Psr\Log\LoggerInterface`.
- `src/Billing/Application/BillOrder.php` takes an `App\Order\Domain\Order`.
- `src/Controller/OrderController.php` takes an `App\Order\Infrastructure\InMemoryOrderRepository`.
- `src/Shipping/Domain/Shipment.php` takes a `Psr\Log\LoggerInterface`. `Shipping` is a context that `deptrac.yaml` does not list.

```console
$ harness lint
error ddd/cross-context src/Billing/Application/BillOrder.php:11 App\Billing\Application\BillOrder must not depend on App\Order\Domain\Order (BillingApplication on OrderDomain)
error ddd/src-roots src/Controller/OrderController.php:1 App\Controller\OrderController is in no layer of deptrac.yaml
error ddd/domain-purity src/Order/Domain/OrderJournal.php:11 App\Order\Domain\OrderJournal must not depend on Psr\Log\LoggerInterface (OrderDomain on Vendor)
error ddd/domain-purity src/Order/Domain/OrderSource.php:11 App\Order\Domain\OrderSource must not depend on Symfony\Component\HttpFoundation\Request (OrderDomain on Framework)
error ddd/src-roots src/Shipping/Domain/Shipment.php:1 App\Shipping\Domain\Shipment is in no layer of deptrac.yaml
error: lint found error findings
$ echo $?
1
```

deptrac checks no dependency of a class that is in no layer, so `OrderController` and `Shipment` give only the `ddd/src-roots` finding. Add a layer for the class, and deptrac then checks its dependencies. The proof then removed the five classes.

A violation with no catalogue rule stops the plugin. The proof added `src/Billing/Infrastructure/KernelProbe.php` that takes an `App\Kernel`:

```console
$ harness lint
jq: error (at <stdin>:1): deptrac violation with no catalogue rule: App\Billing\Infrastructure\KernelProbe must not depend on App\Kernel (BillingInfrastructure on Kernel)
error: lint engine deptrac: plugin failed: deptrac: exit status 5
$ echo $?
1
```

The `Vendor` layer leaves out each class of `src/`, so a class of `src/` outside `App` is in no layer. The proof added `src/Legacy/Thing.php` with the class `Legacy\Thing`:

```console
$ harness lint
error ddd/src-roots src/Legacy/Thing.php:1 Legacy\Thing is in no layer of deptrac.yaml
error: lint found error findings
$ echo $?
1
```

### The canaries pass

`harness lint canaries` sends `lint.check` to the engine of each rule with a canary, in the canary folder, with only that rule. The rule must be in the findings. No agent runs in this check:

```console
$ harness lint canaries
PASS ddd/src-roots lint/canaries/ddd-src-roots
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
PASS ddd/layer-direction lint/canaries/ddd-layer-direction
PASS ddd/cross-context lint/canaries/ddd-cross-context
WAIVED ddd/file-suffix: deptrac checks the dependencies, not the file names.
$ echo $?
0
```

### A config edit breaks a rule

The proof let the application layer of `Order` use the infrastructure layer of `Order`:

```diff
     OrderApplication:
       - OrderDomain
       - SharedDomain
       - DomainLibraries
       - Vendor
+      - OrderInfrastructure
```

deptrac then gave no finding for the canary `ddd-layer-direction`, and the check failed:

```console
$ harness lint canaries
PASS ddd/src-roots lint/canaries/ddd-src-roots
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
FAIL ddd/layer-direction lint/canaries/ddd-layer-direction: the engine deptrac gave no ddd/layer-direction finding
PASS ddd/cross-context lint/canaries/ddd-cross-context
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
PASS ddd/src-roots lint/canaries/ddd-src-roots
PASS ddd/domain-purity lint/canaries/ddd-domain-purity
WAIVED ddd/layer-direction: the team lets the application layer use the infrastructure layer.
PASS ddd/cross-context lint/canaries/ddd-cross-context
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
│   └── plugins
│       ├── deptrac.sh                                      you add, commit
│       └── make-gates.sh                                   you add, commit
├── .harness-lint
│   ├── VERSION                                             harness writes (lint init), commit
│   └── baseline.json                                       harness writes (lint baseline), commit
├── .phpunit.cache
│   └── test-run-history
├── bin
│   ├── console
│   └── phpunit
├── config
│   ├── packages
│   │   ├── cache.yaml
│   │   ├── framework.yaml
│   │   └── routing.yaml
│   ├── routes
│   │   └── framework.yaml
│   ├── bundles.php
│   ├── preload.php
│   ├── reference.php
│   ├── routes.yaml
│   └── services.yaml
├── lint
│   ├── architecture
│   │   └── pack.yaml                                       you add, commit
│   ├── canaries
│   │   ├── ddd-cross-context
│   │   │   └── src
│   │   │       ├── Billing
│   │   │       │   └── Application
│   │   │       │       └── IssueInvoice.php                you add, commit
│   │   │       └── Order
│   │   │           └── Domain
│   │   │               └── Order.php                       you add, commit
│   │   ├── ddd-domain-purity
│   │   │   └── src
│   │   │       └── Order
│   │   │           └── Domain
│   │   │               └── Order.php                       you add, commit
│   │   ├── ddd-layer-direction
│   │   │   └── src
│   │   │       └── Order
│   │   │           ├── Application
│   │   │           │   └── PlaceOrder.php                  you add, commit
│   │   │           └── Infrastructure
│   │   │               └── OrderTable.php                  you add, commit
│   │   └── ddd-src-roots
│   │       └── src
│   │           └── Shipping
│   │               └── Domain
│   │                   └── Shipment.php                    you add, commit
│   └── rules.yaml                                          you add, commit
├── public
│   └── index.php
├── src
│   ├── Billing
│   │   ├── Application
│   │   │   └── IssueInvoiceHandler.php
│   │   ├── Domain
│   │   │   └── Invoice.php
│   │   └── Infrastructure
│   │       └── InvoicePrinter.php
│   ├── Controller
│   │   └── .gitignore
│   ├── Order
│   │   ├── Application
│   │   │   └── PlaceOrderHandler.php
│   │   ├── Domain
│   │   │   ├── Order.php
│   │   │   └── OrderRepository.php
│   │   └── Infrastructure
│   │       └── InMemoryOrderRepository.php
│   └── Kernel.php
├── tests
│   ├── PlaceOrderHandlerTest.php
│   └── bootstrap.php
├── .editorconfig
├── .env
├── .env.dev
├── .env.test
├── .gitignore                                              you add the harness lines, commit
├── .harness.yaml                                           you add the plugins and the pack, harness writes setup and gates (gates init), commit
├── .php-cs-fixer.cache
├── .php-cs-fixer.dist.php                                  you add, commit
├── Makefile                                                you add, commit
├── compose.yaml                                            you add, commit
├── composer.json
├── composer.lock
├── deptrac.yaml                                            you add, commit
├── phpunit.dist.xml
└── symfony.lock

43 directories, 51 files
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
