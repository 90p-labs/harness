package harness

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Two parts, so this file does not match its own search.
const oldRepository = "jorge07" + "/harness"

func TestNoTrackedFileNamesTheOldRepository(t *testing.T) {
	listed, err := exec.Command("git", "ls-files", "-z").Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range strings.Split(string(listed), "\x00") {
		if path == "" {
			continue
		}
		raw, err := os.ReadFile(path)
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
