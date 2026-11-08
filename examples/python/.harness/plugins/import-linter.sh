#!/bin/sh
set -eu

envelope=$(cat)
prefix=$(git rev-parse --show-prefix)
cd "$(git rev-parse --show-toplevel)"

report=$(mktemp)
container="import-linter-$$"
trap 'rm -f "$report"' EXIT
# A TERM stops the compose client, not the container.
trap 'docker kill "$container" >/dev/null 2>&1; exit 143' TERM

# Without a terminal, rich wraps the report at 80 columns. COLUMNS stops the wrap, so each import stays on one line.
docker compose --progress quiet run --rm -T --name "$container" -e COLUMNS=1000 -e PYTHONDONTWRITEBYTECODE=1 \
  -e PYTHONPATH="/app/${prefix}src:/app/.harness/plugins" python \
  uv run --no-sync lint-imports --no-cache --no-logo --config /app/.importlinter </dev/null >"$report" 2>&1 &
wait $! || true
output=$(cat "$report")

if ! printf '%s\n' "$output" | grep -q '^Contracts: [0-9]* kept, [0-9]* broken\.$'; then
  printf '%s\n' "$output" >&2
  exit 1
fi

unnamed=$(printf '%s\n' "$output" | awk '
  /^Contracts: / { exit }
  / (KEPT|BROKEN)( \(.*\))?$/ { sub(/ (KEPT|BROKEN)( \(.*\))?$/, ""); if ($0 !~ /^[a-z0-9-]+\/[a-z0-9-]+$/) print }
')
if [ -n "$unnamed" ]; then
  printf 'import-linter.sh: a contract name must be a rule id, such as ddd/cross-context. These names are not:\n%s\n' "$unnamed" >&2
  exit 1
fi

rules=$(printf '%s\n' "$envelope" | jq -r '[.payload.rules[].id] | join(" ")')
imports=$(printf '%s\n' "$output" | awk -v rules="$rules" '
  BEGIN { split(rules, ids, " "); for (i in ids) reported[ids[i]] = 1 }
  function record(label, importer, lines) { sub(/^\(l\./, "", lines); return sprintf("%s\t%s: %s\t%s\t%d", rule, header, label, importer, lines) }
  function flush() {
    if (found == "") return
    if (links == "" && !heads) { print found; if (rule in reported) direct[pair] = 1 } else chain[found] = links
    found = ""; links = ""; heads = 0
  }
  { above = last; last = $0 }
  /^Broken contracts$/ { section = 1; next }
  !section { next }
  /^-+$/ { flush(); rule = above; unlisted = 0; next }
  /^The following modules are not listed as layers:$/ { flush(); unlisted = 1; next }
  unlisted && $1 == "-" && NF == 2 { printf "%s\t%s is in no context and layer that .importlinter lists\t%s\t1\n", rule, $2, $2; next }
  / is not allowed to import / { flush(); header = $0; sub(/:$/, "", header); next }
  $1 == "-" && $3 == "->" { flush(); pair = $2 " -> " $4; found = record(pair, $2, $5); next }
  $1 == "-" && $3 ~ /^\(l\./ { flush(); heads = 1; found = record($2, $2, $3); next }
  heads && links == "" && $1 == "&" { found = found "\n" ($3 == "->" ? record($2 " -> " $4, $2, $5) : record($2, $2, $3)); next }
  $2 == "->" { links = links SUBSEP $1 " -> " $3; next }
  END {
    flush()
    for (f in chain) {
      n = split(chain[f], link, SUBSEP); covered = 0
      for (i = 2; i <= n; i++) if (link[i] in direct) covered = 1
      if (!covered) print f
    }
  }
')

printf '%s\n%s\n' "$envelope" "$imports" | jq -nR '
  (input | fromjson).payload as $payload
  | {verdict: "allow", findings: [
      inputs | select(. != "") | split("\t")
      | {rule: .[0], message: .[1], path: ("src/" + (.[2] | gsub("\\."; "/"))), line: (.[3] | tonumber)}
      | select(.rule | IN($payload.rules[].id))
      | . as $found
      | ([$found.path + ".py", $found.path + "/__init__.py"] | map(select(IN($payload.files[]))) | first) as $file
      | select($file != null)
      | {rule: $found.rule, file: $file, line: $found.line, message: $found.message}
    ] | unique_by([.rule, .file, .line])}'
