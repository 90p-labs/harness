package process_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/panel/infrastructure/process"
)

func newProcess(dir string) *process.Process {
	p := process.New(process.Config{Dir: dir})
	p.ClaimWait = 150 * time.Millisecond
	return p
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}

func TestClaim(t *testing.T) {
	t.Run("a second claim fails while the first holds the lock", func(t *testing.T) {
		dir := t.TempDir()
		first, err := newProcess(dir).Claim()
		if err != nil {
			t.Fatalf("first claim: %v", err)
		}
		if _, err := newProcess(dir).Claim(); !errors.Is(err, process.ErrClaimed) {
			t.Fatalf("second claim error = %v, want ErrClaimed", err)
		}
		if err := first.Release(); err != nil {
			t.Fatalf("release: %v", err)
		}
		again, err := newProcess(dir).Claim()
		if err != nil {
			t.Fatalf("claim after release: %v", err)
		}
		_ = again.Release()
	})

	t.Run("a claim retries until the holder releases", func(t *testing.T) {
		dir := t.TempDir()
		first, err := newProcess(dir).Claim()
		if err != nil {
			t.Fatal(err)
		}
		time.AfterFunc(100*time.Millisecond, func() { _ = first.Release() })
		p := newProcess(dir)
		p.ClaimWait = 3 * time.Second
		second, err := p.Claim()
		if err != nil {
			t.Fatalf("retrying claim: %v", err)
		}
		_ = second.Release()
	})

	t.Run("state files have mode 0600 and publish leaves no temp file", func(t *testing.T) {
		dir := t.TempDir()
		claim, err := newProcess(dir).Claim()
		if err != nil {
			t.Fatal(err)
		}
		inst := domain.Instance{PID: os.Getpid(), Port: 4321, Version: "v1"}
		if err := claim.Publish(inst); err != nil {
			t.Fatalf("publish: %v", err)
		}
		for _, name := range []string{"panel.json", "panel.lock"} {
			if got := mode(t, filepath.Join(dir, name)); got != 0o600 {
				t.Errorf("%s mode = %o, want 600", name, got)
			}
		}
		raw, err := os.ReadFile(filepath.Join(dir, "panel.json"))
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("panel.json is not json: %v", err)
		}
		if got["pid"] != float64(os.Getpid()) || got["port"] != float64(4321) || got["repo"] != nil {
			t.Errorf("panel.json = %s", raw)
		}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if e.Name() != "panel.json" && e.Name() != "panel.lock" {
				t.Errorf("unexpected file %s", e.Name())
			}
		}
		if err := claim.Release(); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dir, "panel.json")); !os.IsNotExist(err) {
			t.Errorf("panel.json after release: %v, want not exist", err)
		}
	})
}
