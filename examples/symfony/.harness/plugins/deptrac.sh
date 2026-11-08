#!/bin/sh
set -eu

envelope=$(cat)
prefix=$(git rev-parse --show-prefix)
cd "$(git rev-parse --show-toplevel)"

# deptrac resolves its paths from projectDirectory, so a canary folder takes the place of the project.
report=$(docker compose --progress quiet run --rm -T php sh -c '
  config=deptrac.yaml
  if [ -n "$1" ]; then
    config=/tmp/deptrac.canary.yaml
    printf "parameters:\n  projectDirectory: /app/%s\nimports:\n  - /app/deptrac.yaml\n" "$1" > "$config"
  fi
  vendor/bin/deptrac --no-progress --no-cache --config-file="$config" --formatter=json --output=/tmp/report.json >/tmp/deptrac.log 2>&1
  cat /tmp/report.json 2>/dev/null || { cat /tmp/deptrac.log >&2; exit 1; }
' deptrac "$prefix" </dev/null)

printf '%s\n%s\n' "$envelope" "$report" | jq -s --arg base "/app/$prefix" '
  .[0].payload as $payload
  | .[1] as $report
  | {"Domain": "ddd/domain-purity", "Application": "ddd/layer-direction"} as $layerRules
  | if $report.Report.Errors > 0 then error("deptrac reported errors") else . end
  | {verdict: "allow", findings: [
      $report.files | objects | to_entries[]
      | (.key | ltrimstr($base)) as $file
      | select($file | IN($payload.files[]))
      | .value.messages[]
      | $layerRules[.message | capture("\\((?<from>[A-Za-z]+) on [A-Za-z]+\\)$").from] as $rule
      | select($rule != null and ($rule | IN($payload.rules[].id)))
      | {rule: $rule, file: $file, line: .line, message: .message}
    ]}'
