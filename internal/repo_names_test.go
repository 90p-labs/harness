package internal_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	// Two parts, so this file does not match its own search.
	oldRepository = "jorge07" + "/harness"
)

func TestNoTrackedFileNamesTheOldRepository(t *testing.T) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = repository.Root(t)
	listed, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range strings.Split(string(listed), "\x00") {
		if path == "" {
			continue
		}
		raw, err := os.ReadFile(repository.Path(t, path))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		for i, line := range bytes.Split(raw, []byte("\n")) {
			if bytes.Contains(line, []byte(oldRepository)) {
				t.Errorf("%s:%d names %s, want the 90p-labs/harness repository", path, i+1, oldRepository)
			}
		}
	}
}
