# Plugins

This page holds the plugin contract and one example for each plugin kind, and the [README](../README.md) links here.

A plugin is a command in the `plugins` key. The harness writes the event envelope as JSON on stdin and reads one verdict from stdout: `{"verdict":"allow|deny|context","reason":"...","context":"...","findings":[]}`. An empty stdout is an allow, and a non-zero exit is a plugin failure. The default `timeout` is 5 seconds. [`DESIGN.md`](../DESIGN.md#plugin-events) gives the full contract: each event, its kind and its payload. The output below is real, from scratch repositories. Two scripts need `jq`.

## Deploy tool plugin

A deploy tool plugin answers `gates.detect` with the gates that it proposes, one JSON gate on each line of `context`. `harness gates init` and `harness init` append them. `scripts/kubeconform-detect.sh` proposes a kubeconform gate when the repository has files in `deploy/manifests/`:

```sh
#!/bin/sh
if grep -q '"deploy/manifests/'; then
  echo '{"verdict":"allow","context":"{\"id\":\"kubeconform\",\"run\":\"kubeconform -summary deploy/manifests\",\"paths\":[\"deploy/manifests/**\"]}"}'
fi
```

```console
$ cat .harness.yaml
plugins:
  - {name: kubeconform, command: [sh, scripts/kubeconform-detect.sh], events: [gates.detect]}
$ harness gates init
helm-render: helm template deploy/chart > /dev/null
kubeconform: kubeconform -summary deploy/manifests
```

The default plugin `helm` proposed `helm-render` for `deploy/chart/Chart.yaml`. `deploy.disabled` turns off a default plugin, `helm` or `promtool`. With `deploy: {disabled: [helm]}`, the same command proposes only the kubeconform gate.

## Custom lint pack and lint engine plugin

An entry of `lint.packs` with a `/` is a custom pack: a folder with a `pack.yaml`. A rule names its engine: `biome-grit`, `biome-rule`, `oxlint` or a lint plugin. [`DESIGN.md`](../DESIGN.md#custom-packs) gives the GritQL rules and the other keys. A lint engine plugin answers `lint.check`. The payload holds the rules of its engine and the files that match their scopes. The findings of the verdict go through the same report, baseline ratchet and lint gate as the built-in rules. `scripts/console-lint.sh`:

```sh
#!/bin/sh
jq -j '.payload.files[] + "\u0000"' | xargs -0 grep -IHn 'console\.log' /dev/null |
  jq -Rn '{verdict: "allow", findings: [inputs | split(":") | {rule: "team/no-console", file: .[0], line: (.[1] | tonumber), message: "console.log in source"}]}'
```

```console
$ cat lint/team/pack.yaml
name: team
rules:
  - id: team/no-console
    engine: console-lint
    severity: error
    scope: "src/**/*.ts"
    message: A source file calls console.log. Use the logger.
$ cat .harness.yaml
lint: {packs: [./lint/team]}
plugins:
  - {name: console-lint, command: [sh, scripts/console-lint.sh], events: [lint.check], timeout: 30s}
$ harness lint init
$ harness lint
error team/no-console src/checkout.ts:2 console.log in source
error: lint found error findings
```

Run `harness lint init` after each edit of a custom pack. A finding takes the severity of its rule. `lint.disabled` turns off a default engine (`harness`, `biome-grit` or `oxlint`) or one rule, such as `next/no-lib-folder`. The rules of a turned-off engine do not run: for example, `lint: {disabled: [biome-grit]}`.

## Architecture rules in any language

The built-in `ddd` pack checks the layers and the contexts of a TypeScript project. For TypeScript, a canary is optional: the binary tests prove the built-in rules, and with no `rules.yaml` in the proof folder, `harness lint canaries --proof lint/proof` prints `lint canaries: only built-in rules are active, so there is no canary to run`. For PHP, Python or Go, `harness init` prints a next step, such as `next: run the lint-setup skill to set up the architecture rules for PHP`. The `lint-setup` skill asks the open choices, and the `linter` agent writes the setup with the tool of the language: deptrac, import-linter, go-arch-lint or depguard. It writes the tool config, a wrapper lint plugin, a custom pack with the ids of `harness lint catalogue`, and a temporary proof folder `lint/proof/` with one canary folder that breaks each rule and a `rules.yaml`. The skill deletes the proof folder after the proof, so the project commits no canary. [`DESIGN.md`](../DESIGN.md#architecture-rules-in-any-language) gives the formats.

`harness lint canaries --proof lint/proof` proves that the tool catches each rule. The flag is required. A rule that the tool cannot catch shows its waiver, and the skill reports each waiver to the user:

```console
$ harness lint canaries --proof lint/proof
PASS ddd/src-roots ddd-src-roots
PASS ddd/domain-purity ddd-domain-purity
PASS ddd/layer-direction ddd-layer-direction
FAIL ddd/cross-context ddd-cross-context: the engine fake-arch gave no ddd/cross-context finding
PASS ddd/file-suffix ddd-file-suffix
error: a lint canary failed
$ harness lint canaries --proof lint/proof
PASS ddd/src-roots ddd-src-roots
PASS ddd/domain-purity ddd-domain-purity
PASS ddd/layer-direction ddd-layer-direction
WAIVED ddd/cross-context: the tool cannot see the context boundaries in this project.
PASS ddd/file-suffix ddd-file-suffix
```

A canary passes only when the finding is for a file inside the canary folder. `ddd/src-roots` makes the tool report each file or dependency that it cannot place, so no source folder drops out of the check. A waiver needs a reason of 3 words or more, and a proof that waives each rule fails. On a `FAIL`, the skill stops and keeps `lint/proof/`. On success, it deletes the folder and reports each `PASS` and each `WAIVED` line. A later edit of the tool config that breaks a rule stays green until someone runs the skill again.

A repository with two languages runs the skill once for each language. `harness init` prints a `next:` line for each language that no plugin rule covers. Run the `lint-setup` skill for each language: the second pack holds the same catalogue ids on its own engine, and its scope must not hold a file of the first. A rule on two engines gives one canary or one waiver for each engine under `engines`, and each line names its engine. This is a TypeScript and PHP repository with the built-in `ddd` pack and a deptrac pack:

```console
$ harness lint canaries --proof lint/proof
PASS ddd/src-roots deptrac ddd-src-roots
PASS ddd/src-roots harness ts-src-roots
PASS ddd/domain-purity deptrac ddd-domain-purity
PASS ddd/domain-purity harness ts-domain-purity
PASS ddd/layer-direction deptrac ddd-layer-direction
PASS ddd/layer-direction harness ts-layer-direction
PASS ddd/cross-context deptrac ddd-cross-context
WAIVED ddd/cross-context harness: the binary fixtures prove the built-in rule.
WAIVED ddd/file-suffix: the binary fixtures prove the built-in rule.
$ harness lint
error ddd/domain-purity src/Order/Domain/Order.php:9 App\Order\Domain\Order must not depend on Symfony\Component\Uid\Uuid (OrderDomain on Framework)
error ddd/domain-purity src/contexts/order/domain/clock.entity.ts:1 A domain file imports a package that is not allowed, or an application or infrastructure path.
error: lint found error findings
```

## Signals plugin

In `observing`, `harness live check` sends one `signals.check` event for each row of the `## Signals` table of the plan. A plugin answers only the rows of its own `source`, and a breach is a deny. [`docs/go-live.md`](go-live.md) gives the table, the payload and the verdict. `scripts/prom-check.sh` reads the value of the query at the check time from `$PROMETHEUS_URL`, which must be set where `harness live check` runs. A failed read is a plugin failure:

```sh
#!/bin/sh
row=$(jq -c .payload)
[ "$(echo "$row" | jq -r .source)" = prometheus ] || exit 0
value=$(curl -sfG "$PROMETHEUS_URL/api/v1/query" --data-urlencode "query=$(echo "$row" | jq -r .query)" | jq -er '.data.result[0].value[1]') || exit 1
echo "$row" | jq --argjson v "$value" '.breach as $b | select({">": ($v > $b.value), ">=": ($v >= $b.value), "<": ($v < $b.value), "<=": ($v <= $b.value)}[$b.op]) | {verdict: "deny", reason: "\(.signal) \($v) \($b.op) \($b.value)"}'
```

The `plugins` entry is `{name: prom, command: [sh, scripts/prom-check.sh], events: [signals.check]}`. In this scratch run, a stub `gh` gave the merge and the deploy, and the first check moved the run to `observing`. On the next check, the breach sends the run back to `planning` with the findings:

```console
$ harness live check
20261108-161558-let-a-customer-cancel-an-order observing regression
$ harness run status
run 20261108-161558-let-a-customer-cancel-an-order phase=planning review_round=0 live_round=1
reason: Regression in live round 1 of run 20261108-161558-let-a-customer-cancel-an-order: PR https://github.com/acme/orders/pull/7, sha 4f2a9c1, production.
- prom: error rate 0.034 > 0.01, window 30m, panel https://grafana.example/d/orders
...
```

## Hook plugin

A hook plugin works on a host hook event, such as `tool.before`, or on a loop event, such as `plan.submitted` or `run.transition.before`. `scripts/plan-has-risks.sh` refuses a plan with no `## Risks` section:

```sh
#!/bin/sh
plan=$(sed -n 's/.*"path":"\([^"]*\)".*/\1/p')
grep -q '^## Risks' "$plan" || echo '{"verdict":"deny","reason":"the plan has no ## Risks section"}'
```

The `plugins` entry is `{name: plan-has-risks, command: [sh, scripts/plan-has-risks.sh], events: [plan.submitted]}`:

```console
$ harness run plan
error: plan rejected: plan-has-risks: the plan has no ## Risks section
```

A plugin on `run.transition.before` or `run.transition.after` can run for chosen transitions only. Add the keys `transition`, `from` or `to` to the entry: see [Filter a transition](configuration.md#filter-a-transition).
