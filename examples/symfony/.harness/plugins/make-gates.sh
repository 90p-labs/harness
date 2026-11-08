#!/bin/sh
set -eu

PATHS='"**/*.php","src/**","tests/**","config/**","bin/**","public/**","templates/**","translations/**","composer.json","composer.lock",".env*","phpunit.dist.xml","phpunit.xml.dist","phpstan.neon","phpstan.dist.neon","phpstan.neon.dist","phpstan-baseline.neon",".php-cs-fixer.dist.php","Makefile","compose.yaml"'
SETUP_PATHS='"composer.json","composer.lock","compose.yaml","Makefile"'

awk -v paths="$PATHS" -v setup_paths="$SETUP_PATHS" '
/^[A-Za-z0-9_-]+:.*## (gate|setup)([ \t].*)?$/ {
  target = $0
  sub(/:.*/, "", target)
  kind = $0
  sub(/.*## /, "", kind)
  sub(/[ \t].*/, "", kind)
  marker = $0
  sub(/.*## (gate|setup)[ \t]*/, "", marker)
  sub(/[ \t]+$/, "", marker)
  inputs = paths
  extra = ""
  if (kind == "setup") {
    inputs = setup_paths
    extra = ",\"setup\":true"
    if (marker ~ /^creates:/) {
      sub(/^creates:[ \t]*/, "", marker)
      extra = extra ",\"creates\":\"" marker "\""
      marker = ""
    }
  } else if (marker ~ /^needs:/) {
    sub(/^needs:[ \t]*/, "", marker)
    n = split(marker, needs, /[ \t]+/)
    list = ""
    for (i = 1; i <= n; i++) list = list (i > 1 ? "," : "") "\"make-" needs[i] "\""
    extra = ",\"needs\":[" list "]"
    marker = ""
  }
  if (marker != "") {
    print "make-gates: target " target ": unknown marker \"" marker "\"" > "/dev/stderr"
    failed = 1
    exit 1
  }
  line = "{\"id\":\"make-" target "\",\"run\":\"make " target "\",\"paths\":[" inputs "]" extra "}"
  gsub(/"/, "\\\"", line)
  context = context (count++ ? "\\n" : "") line
}
END {
  if (failed) exit 1
  printf "{\"verdict\":\"allow\",\"context\":\"%s\"}\n", context
}
' Makefile
