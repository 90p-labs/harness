#!/bin/sh
set -eu

envelope=$(cat)
report=$(docker compose --progress quiet run --rm -T python uv run --locked --exact ruff check --no-cache --output-format json --exit-zero . </dev/null)

printf '%s\n%s\n' "$envelope" "$report" | jq -s '.[0].payload as $payload | {
  verdict: "allow",
  findings: [.[1][] | (.filename | ltrimstr("/app/")) as $file
    | select($file | IN($payload.files[]))
    | {rule: $payload.rules[0].id, file: $file, line: .location.row, message: ([.code, .message] | map(values) | join(" "))}]
}'
