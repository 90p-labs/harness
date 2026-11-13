package docs_test

import (
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const secretsReadRowPrefix = "| `secrets-read` |"

var secretsReadNames = []string{
	"`env`",
	"`printenv`",
	"`export`",
	"`export -p`",
	"`set`",
	"`env` whose `-S` string, split into words, leaves no command",
	"bare `declare` or `typeset`, either with `-p`, or with other flags and no name (not `-f` or `-F`)",
	"`/proc/<pid>/environ`",
}

func secretsReadRow(t *testing.T, file string) string {
	t.Helper()
	var found []string
	for _, line := range strings.Split(repository.Read(t, file), "\n") {
		if strings.HasPrefix(line, secretsReadRowPrefix) {
			found = append(found, line)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s has %d secrets-read rows, want 1", file, len(found))
	}
	return found[0]
}

func TestSecretsReadDocsRow(t *testing.T) {
	config := secretsReadRow(t, configDocFile)
	design := secretsReadRow(t, designFile)
	if config != design {
		t.Errorf("secrets-read rows differ:\n%s: %s\n%s: %s", configDocFile, config, designFile, design)
	}
	for _, name := range secretsReadNames {
		if !strings.Contains(config, name) {
			t.Errorf("secrets-read row does not name %s", name)
		}
	}
}
