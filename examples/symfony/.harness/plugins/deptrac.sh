#!/bin/sh
set -eu

envelope=$(cat)
prefix=$(git rev-parse --show-prefix)
cd "$(git rev-parse --show-toplevel)"

# A signal stops the compose client, not the container, so the trap kills the container by name.
name="deptrac-lint-$$"
report=$(mktemp)
trap 'rm -f "$report"' EXIT
trap 'docker kill "$name" >/dev/null 2>&1; exit 143' TERM

# deptrac resolves its paths from projectDirectory, so a canary folder takes the place of the project.
# debug:unassigned exits 2 when it lists a class, and prints a sentence, not a class, when it exits 0.
docker compose --progress quiet run --rm -T --name "$name" php sh -c '
  config=deptrac.yaml
  if [ -n "$1" ]; then
    config=/tmp/deptrac.canary.yaml
    printf "parameters:\n  projectDirectory: /app/%s\nimports:\n  - /app/deptrac.yaml\n" "$1" > "$config"
  fi
  vendor/bin/deptrac --no-progress --no-cache --report-uncovered --config-file="$config" --formatter=json --output=/tmp/report.json >/tmp/deptrac.log 2>&1
  cat /tmp/report.json 2>/dev/null || { cat /tmp/deptrac.log >&2; exit 1; }
  vendor/bin/deptrac debug:unassigned --no-cache --config-file="$config" >/tmp/unassigned.txt 2>/tmp/deptrac.log && : >/tmp/unassigned.txt || [ $? -eq 2 ] || { cat /tmp/deptrac.log >&2; exit 1; }
  php -r "echo json_encode(file(\"/tmp/unassigned.txt\", FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES));"
' deptrac "$prefix" </dev/null >"$report" &
wait $!

printf '%s\n' "$envelope" | jq -s --slurpfile report "$report" --arg base "/app/$prefix" '
  def layer: capture("^(?<context>.+?)(?<kind>Domain|Application|Infrastructure)$")? // {};
  .[0].payload as $payload
  | $report[0] as $deptrac
  | $report[1] as $unassigned
  | {"Domain": "ddd/domain-purity", "Application": "ddd/layer-direction"} as $kindRules
  | "ddd/src-roots" as $srcRoots
  | if $deptrac.Report.Errors > 0 then error("deptrac reported errors") else . end
  | ([
      $deptrac.files | objects | to_entries[]
      | (.key | ltrimstr($base)) as $file
      | .value.messages[]
      | .message as $message
      | (($message | capture("\\((?<from>[^ ()]+)(?: on (?<to>[^ ()]+))?\\)$")?)
         // error("deptrac message with no layer: " + $message)) as $layers
      | ($layers.from | layer) as $from
      | (($layers.to // "") | layer) as $to
      | (if $layers.to == null then $srcRoots
         elif $from.context != null and $to.context != null and $from.context != $to.context then "ddd/cross-context"
         else $kindRules[$from.kind // ""] end) as $rule
      | {rule: $rule, file: $file, line: .line, message: .message}
    ] + [
      $unassigned[]
      | {rule: $srcRoots, file: ("src/" + (ltrimstr("App\\") | gsub("\\\\"; "/")) + ".php"), line: 1,
         message: "\(.) is in no layer of deptrac.yaml"}
    ]
    | map(select(.file | IN($payload.files[])))) as $findings
  | ([$findings[] | select(.rule == null) | .message]) as $unmapped
  | if ($unmapped | length) > 0
    then error("deptrac violation with no catalogue rule: " + ($unmapped | join("; ")))
    else . end
  | {verdict: "allow", findings: [$findings[] | select(.rule | IN($payload.rules[].id))]}'
