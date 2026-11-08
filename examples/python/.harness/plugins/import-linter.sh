#!/bin/sh
set -eu

envelope=$(cat)
prefix=$(git rev-parse --show-prefix)
cd "$(git rev-parse --show-toplevel)"

# Without a terminal, rich wraps the report at 80 columns. COLUMNS stops the wrap, so each import stays on one line.
output=$(docker compose --progress quiet run --rm -T -e COLUMNS=1000 -e PYTHONPATH="/app/${prefix}src" python \
  uv run --locked --exact lint-imports --no-cache --no-logo --config /app/.importlinter </dev/null 2>&1) || true

if ! printf '%s\n' "$output" | grep -q '^Contracts: [0-9]* kept, [0-9]* broken\.$'; then
  printf '%s\n' "$output" >&2
  exit 1
fi

imports=$(printf '%s\n' "$output" | awk '
  /^Broken contracts$/ { section = 1; next }
  !section { next }
  /^[a-z-]+\/[a-z-]+$/ { rule = $0; next }
  / is not allowed to import / { header = $0; sub(/:$/, "", header); next }
  $1 == "-" && $3 == "->" { line = $5; sub(/^\(l\./, "", line); line += 0; printf "%s\t%s\t%s\t%s\t%s\n", rule, header, $2, $4, line }
')

printf '%s\n%s\n' "$envelope" "$imports" | jq -nR '
  (input | fromjson).payload as $payload
  | {verdict: "allow", findings: [
      inputs | select(. != "") | split("\t")
      | {rule: .[0], message: "\(.[1]): \(.[2]) -> \(.[3])", path: ("src/" + (.[2] | gsub("\\."; "/"))), line: (.[4] | tonumber)}
      | select(.rule | IN($payload.rules[].id))
      | . as $found
      | ([$found.path + ".py", $found.path + "/__init__.py"] | map(select(IN($payload.files[]))) | first) as $file
      | select($file != null)
      | {rule: $found.rule, file: $file, line: $found.line, message: $found.message}
    ] | unique_by([.rule, .file, .line])}'
