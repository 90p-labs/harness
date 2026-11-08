#!/bin/sh
set -eu

envelope=$(cat)
prefix=$(git rev-parse --show-prefix)
cd "$(git rev-parse --show-toplevel)"

report=$(docker compose --progress quiet run --rm -T go sh -c '
  out=$(go run github.com/fe3dback/go-arch-lint@v1.19.1 check --json --arch-file /src/.go-arch-lint.yml --project-path "/src/$1" 2>/tmp/go-arch-lint.log)
  case "$out" in
    "{"*) printf "%s\n" "$out" ;;
    *) cat /tmp/go-arch-lint.log >&2; exit 1 ;;
  esac
' go-arch-lint "$prefix" </dev/null)

printf '%s\n%s\n' "$envelope" "$report" | jq -s '
  .[0].payload as $payload
  | .[1].Payload as $report
  | {"domain": "ddd/domain-purity", "application": "ddd/layer-direction"} as $layerRules
  | if ($report.ExecutionWarnings | length) > 0
    then error("go-arch-lint config: " + ([$report.ExecutionWarnings[].Text] | join("; ")))
    else . end
  | {verdict: "allow", findings: [
      $report.ArchWarningsDeps[]
      | (.FileRelativePath | ltrimstr("/")) as $file
      | select($file | IN($payload.files[]))
      | ($file | capture("^internal/(?<context>[^/]+)/(?<layer>[^/]+)/")) as $from
      | ((.ResolvedImportName | capture("/internal/(?<context>[^/]+)/")?) // {}) as $to
      | (if $to.context != null and $to.context != $from.context
         then "ddd/cross-context"
         else $layerRules[$from.layer] end) as $rule
      | select($rule != null and ($rule | IN($payload.rules[].id)))
      | {rule: $rule, file: $file, line: .Reference.Line,
         message: "\(.ComponentName) must not depend on \(.ResolvedImportName)"}
    ]}'
