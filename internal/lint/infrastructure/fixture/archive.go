package fixture

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	headerPrefix = "-- "
	headerSuffix = " --"
	dirMode      = 0o755
	fileMode     = 0o600
)

func Extract(t testing.TB, archive string) string {
	t.Helper()
	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatalf("read archive: %v", err)
	}
	dir := t.TempDir()
	var name string
	var content strings.Builder
	flush := func() {
		defer content.Reset()
		if name == "" {
			return
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content.String()), fileMode); err != nil {
			t.Fatal(err)
		}
	}
	for _, line := range strings.SplitAfter(string(raw), "\n") {
		trimmed := strings.TrimSuffix(line, "\n")
		if strings.HasPrefix(trimmed, headerPrefix) && strings.HasSuffix(trimmed, headerSuffix) && len(trimmed) > len(headerPrefix+headerSuffix) {
			flush()
			name = strings.TrimSpace(trimmed[len(headerPrefix) : len(trimmed)-len(headerSuffix)])
			continue
		}
		content.WriteString(line)
	}
	flush()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	return dir
}
