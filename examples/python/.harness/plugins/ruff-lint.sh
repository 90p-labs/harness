#!/bin/sh
set -eu

envelope=$(cat)
report=$(mktemp)
container="ruff-$$"
trap 'rm -f "$report"' EXIT
# A TERM stops the compose client, not the container.
trap 'docker kill "$container" >/dev/null 2>&1; exit 143' TERM

# A pack scope is one glob with no exclusion, so ruff itself leaves out the canaries.
docker compose --progress quiet run --rm -T --name "$container" python uv run --no-sync \
  ruff check --no-cache --output-format json --exit-zero --extend-exclude lint/canaries . </dev/null >"$report" &
wait $!

printf '%s\n' "$envelope" | jq --slurpfile ruff "$report" '.payload as $payload | {
  verdict: "allow",
  findings: [$ruff[0][] | (.filename | ltrimstr("/app/")) as $file
    | select($file | IN($payload.files[]))
    | {rule: $payload.rules[0].id, file: $file, line: .location.row, message: ([.code, .message] | map(values) | join(" "))}]
}'
