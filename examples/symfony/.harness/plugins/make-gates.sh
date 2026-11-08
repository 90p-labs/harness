#!/bin/sh
set -eu

PATHS='"**/*.php","src/**","tests/**","config/**","bin/**","public/**","templates/**","translations/**","composer.json","composer.lock",".env*","phpunit.dist.xml","phpunit.xml.dist","phpstan.neon","phpstan.dist.neon","phpstan.neon.dist","phpstan-baseline.neon",".php-cs-fixer.dist.php","Makefile","compose.yaml"'

awk -v paths="$PATHS" '
/^[A-Za-z0-9_-]+:.*## gate[ \t]*$/ {
  target = $0
  sub(/:.*/, "", target)
  line = "{\"id\":\"make-" target "\",\"run\":\"make " target "\",\"paths\":[" paths "]}"
  gsub(/"/, "\\\"", line)
  context = context (count++ ? "\\n" : "") line
}
END { printf "{\"verdict\":\"allow\",\"context\":\"%s\"}\n", context }
' Makefile
