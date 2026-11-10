# Full configuration

This page shows a complete `.harness.yaml` and one example for each common case. [configuration.md](configuration.md) gives the type, the env variable and the effect of each key.

## The full file

This file sets each key. Most values are the defaults. A sample value stands where the default is an empty list.

```yaml
state:
  dsn: sqlite:.harness/state.db
run:
  approval: true
  go_live: false
  max_review_rounds: 3
  max_live_rounds: 3
  max_stop_blocks: 3
  strict_minor: false
plugins:
  - name: notify-approval
    command: [scripts/notify.sh]
    events: [run.transition.after]
    timeout: 5s
    transition: [submit-plan]
    from: [planning]
    to: [awaiting_approval]
roles:
  lead:
    claude: [opus]
    codex: [gpt-6-astra]
  explorer:
    claude: [haiku]
    codex: [gpt-6-luna]
  worker:
    claude: [sonnet, haiku]
    codex: [gpt-6.1-sol, gpt-6-luna]
  reviewer:
    claude: [sonnet, haiku, opus]
    codex: [gpt-6.1-sol, gpt-6.1-terra, gpt-6-astra]
  advisor:
    claude: [fable]
    codex: [gpt-6-astra]
guard:
  disabled: [pr-draft-only]
deploy:
  disabled: [promtool]
git:
  default_branch: main
setup:
  - id: node-install
    run: pnpm install --frozen-lockfile
    paths: [package.json, pnpm-lock.yaml]
    creates: node_modules
gates:
  - id: unit
    run: pnpm test
    paths: ["src/**", package.json]
    timeout: 10m
    baseline: .harness-lint/baseline.json
    needs: [lint]
  - id: lint
    run: harness lint --json
verify:
  parallel: 1
lint:
  packs: [ddd]
  disabled: [ddd/domain-purity]
  ddd:
    root: contexts
    domain_allow: [uuid]
    src_files: [main.ts]
  shadcn:
    ui_allow: [clsx]
  observability:
    chart: deploy/chart
    rules: deploy/**/*.rules.yaml
    dashboards: deploy/**/dashboards/*.json
live:
  environment: production
  rollback: helm rollback api
```

## Go

A Go project that runs in Docker. The `lint` gate reads the architecture pack and fails only on a new finding.

```yaml
plugins:
  - name: go-arch-lint
    command: [sh, .harness/plugins/go-arch-lint.sh]
    events: [lint.check]
    timeout: 60s
lint:
  packs: [./lint/architecture]
verify:
  parallel: 2
setup:
  - id: go-mod
    run: docker compose run --rm -T go go mod download
    paths: [go.mod, go.sum, compose.yaml]
gates:
  - id: go-vet
    run: docker compose run --rm -T go go vet ./...
    paths: ["**/*.go", go.mod, go.sum, compose.yaml]
  - id: go-test
    run: docker compose run --rm -T go go test ./...
    paths: ["**/*.go", go.mod, go.sum, compose.yaml]
    needs: [go-vet]
  - id: lint
    run: harness lint --json
    baseline: .harness-lint/baseline.json
```

## Next.js with DDD and shadcn

The pack `nextjs-ddd-shadcn` needs the packs `ddd`, `nextjs` and `shadcn`. The keys `lint.ddd` and `lint.shadcn` tune them.

```yaml
lint:
  packs: [ddd, nextjs, shadcn, nextjs-ddd-shadcn]
  ddd:
    root: modules
    domain_allow: [uuid, date-fns]
    src_files: [main.ts, middleware.ts]
  shadcn:
    ui_allow: [clsx, tailwind-merge]
verify:
  parallel: 3
setup:
  - id: node-install
    run: docker compose run --rm -T node pnpm install --frozen-lockfile
    paths: [package.json, pnpm-lock.yaml, compose.yaml]
gates:
  - id: node-typecheck
    run: docker compose run --rm -T node sh -c 'pnpm exec next typegen && pnpm exec tsc --noEmit'
    paths: ["**/*.ts", "**/*.tsx", pnpm-lock.yaml]
  - id: lint
    run: harness lint --json
    baseline: .harness-lint/baseline.json
  - id: node-build
    run: docker compose run --rm -T node pnpm run build
    paths: ["**/*.ts", "**/*.tsx", "**/*.css", pnpm-lock.yaml]
    needs: [node-typecheck, lint]
```

## Python

A Python project with a folder pack for the language rules and one for the architecture. The plugins run the engines.

```yaml
plugins:
  - name: ruff
    command: [sh, .harness/plugins/ruff-lint.sh]
    events: [lint.check]
    timeout: 60s
  - name: import-linter
    command: [sh, .harness/plugins/import-linter.sh]
    events: [lint.check]
    timeout: 60s
lint:
  packs: [./lint/python, ./lint/architecture]
verify:
  parallel: 2
setup:
  - id: py-install
    run: docker compose run --rm -T python uv sync --locked
    paths: [pyproject.toml, uv.lock, compose.yaml]
gates:
  - id: py-typecheck
    run: docker compose run --rm -T python uv run --no-sync mypy src tests
    paths: ["**/*.py", pyproject.toml, uv.lock]
  - id: lint
    run: harness lint --json
    baseline: .harness-lint/baseline.json
  - id: py-test
    run: docker compose run --rm -T python uv run --no-sync pytest
    paths: ["**/*.py", pyproject.toml, uv.lock]
    needs: [py-typecheck, lint]
```

## Symfony

A Symfony project with the Deptrac engine. The gates come from the targets of a Makefile.

```yaml
plugins:
  - name: deptrac
    command: [sh, .harness/plugins/deptrac.sh]
    events: [lint.check]
    timeout: 60s
lint:
  packs: [./lint/architecture]
setup:
  - id: install
    run: make install
    paths: [composer.json, composer.lock]
    creates: vendor/autoload.php
gates:
  - id: cs
    run: make cs
    paths: ["src/**", "tests/**", composer.lock]
  - id: phpstan
    run: make phpstan
    paths: ["src/**", "tests/**", composer.lock]
  - id: lint
    run: harness lint --json
    baseline: .harness-lint/baseline.json
  - id: test
    run: make test
    paths: ["src/**", "tests/**", composer.lock]
    needs: [cs, phpstan, lint]
```

## Codex

Codex sends no model in an agent call, so the rule `review-models` always denies `harness run ship`. Turn it off. The roles show the default Codex models.

```yaml
guard:
  disabled: [review-models]
roles:
  lead:
    codex: [gpt-6-astra]
  explorer:
    codex: [gpt-6-luna]
  worker:
    codex: [gpt-6.1-sol, gpt-6-luna]
  reviewer:
    codex: [gpt-6.1-sol, gpt-6.1-terra, gpt-6-astra]
  advisor:
    codex: [gpt-6-astra]
```

## Deploy tool plugin

A plugin on `gates.detect` proposes gates for your own deploy tool. The setting `deploy.disabled` turns off the default `helm` plugin.

```yaml
deploy:
  disabled: [helm]
plugins:
  - name: kustomize-gates
    command: [sh, .harness/plugins/kustomize-gates.sh]
    events: [gates.detect]
    timeout: 10s
```

## Live loop

The run follows the change to production. A plugin on `signals.check` reports the health signals. The key `live.rollback` names the command that the findings show on a regression.

```yaml
run:
  go_live: true
  max_live_rounds: 2
plugins:
  - name: signals
    command: [scripts/signals.sh]
    events: [signals.check]
    timeout: 30s
live:
  environment: production
  rollback: helm rollback api
```

## Disabled guard rule

This team merges its own pull requests, so it turns off `pr-no-merge`. Write the reason in the commit message that changes the file.

```yaml
guard:
  disabled: [pr-no-merge]
```

## Custom lint pack and engine plugin

A folder pack holds the rules. A plugin on `lint.check` runs the engine and reports the findings of those rules.

```yaml
lint:
  packs: [ddd, ./lint/architecture]
plugins:
  - name: arch-engine
    command: [sh, .harness/plugins/arch-engine.sh]
    events: [lint.check]
    timeout: 60s
```

The file `lint/architecture/pack.yaml` lists the rules:

```text
name: architecture
rules:
  - id: arch/layer-direction
    engine: arch-engine
    severity: error
    scope: "src/**/*.go"
    message: The application layer uses the domain layer, never the infrastructure layer.
```

## Plugin that filters transitions

The plugin runs on `run.transition.before` and only for the transition `deploy`. A deny from the plugin stops the deploy.

```yaml
plugins:
  - name: predeploy-check
    command: [scripts/predeploy.sh]
    events: [run.transition.before]
    transition: [deploy]
```

## Monorepo with several languages

Each folder has its own pack. The gate ids start with the language, and the paths stay in the folder of the language.

```yaml
lint:
  packs: [./services/api/lint, ./web/lint]
verify:
  parallel: 2
gates:
  - id: go-test
    run: cd services/api && go test ./...
    paths: ["services/api/**/*.go", services/api/go.mod, services/api/go.sum]
  - id: ts-typecheck
    run: cd web && pnpm exec tsc --noEmit
    paths: ["web/**/*.ts", "web/**/*.tsx", web/pnpm-lock.yaml]
  - id: ts-test
    run: cd web && pnpm test
    paths: ["web/**/*.ts", "web/**/*.tsx", web/pnpm-lock.yaml]
    needs: [ts-typecheck]
```
