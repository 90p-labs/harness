#!/bin/sh
set -eu

envelope=$(cat)
prefix=$(git rev-parse --show-prefix)
cd "$(git rev-parse --show-toplevel)"

# A signal stops the compose client, not the container, so the trap kills the container by name.
name="go-arch-lint-$$"
report=$(mktemp)
trap 'rm -f "$report"' EXIT
trap 'docker kill "$name" >/dev/null 2>&1; exit 143' TERM

docker compose --progress quiet run --rm -T --name "$name" go sh -c '
  out=$(go run github.com/fe3dback/go-arch-lint@v1.19.1 check --json --arch-file /src/.go-arch-lint.yml --project-path "/src/$1" 2>/tmp/go-arch-lint.log)
  case "$out" in
    "{"*) printf "%s\n" "$out" ;;
    *) cat /tmp/go-arch-lint.log >&2; exit 1 ;;
  esac
' go-arch-lint "$prefix" </dev/null >"$report" &
wait $!

printf '%s\n' "$envelope" | jq -s --slurpfile report "$report" '
  .[0].payload as $payload
  | $report[0].Payload as $report
  | {"domain": "ddd/domain-purity", "application": "ddd/layer-direction"} as $layerRules
  | "ddd/src-roots" as $srcRoots
  | if ($report.ExecutionWarnings | length) > 0
    then error("go-arch-lint config: " + ([$report.ExecutionWarnings[].Text] | join("; ")))
    else . end
  | ([
      $report.ArchWarningsDeps[]
      | (.FileRelativePath | ltrimstr("/")) as $file
      | (($file | capture("^internal/(?<context>[^/]+)/(?<layer>[^/]+)/")?) // {}) as $from
      | ((.ResolvedImportName | capture("/internal/(?<context>[^/]+)/")?) // {}) as $to
      | (if $from.context != null and $to.context != null and $to.context != $from.context
         then "ddd/cross-context"
         else $layerRules[$from.layer // ""] end) as $rule
      | {rule: $rule, file: $file, line: .Reference.Line,
         message: "\(.ComponentName) must not depend on \(.ResolvedImportName)"}
    ] + [
      $report.ArchWarningsNotMatched[]
      | (.FileRelativePath | ltrimstr("/")) as $file
      | {rule: $srcRoots, file: $file, line: 1,
         message: "\($file) is in no component of .go-arch-lint.yml"}
    ]
    | map(select(.file | IN($payload.files[])))) as $findings
  | ([$findings[] | select(.rule == null) | .message]) as $unmapped
  | if ($unmapped | length) > 0
    then error("go-arch-lint violation with no catalogue rule: " + ($unmapped | join("; ")))
    else . end
  | {verdict: "allow", findings: [$findings[] | select(.rule | IN($payload.rules[].id))]}'
