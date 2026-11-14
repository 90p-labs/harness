package process_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/90p-labs/harness/internal/panel/infrastructure/process"
)

func TestUserDir(t *testing.T) {
	t.Run("an absolute HARNESS_HOME is created with mode 0700", func(t *testing.T) {
		home := filepath.Join(t.TempDir(), "nested", "state")
		t.Setenv("HARNESS_HOME", home)
		got, err := process.UserDir()
		if err != nil {
			t.Fatalf("UserDir: %v", err)
		}
		if got != home {
			t.Fatalf("UserDir = %q, want %q", got, home)
		}
		if m := mode(t, home); m != 0o700 {
			t.Fatalf("mode = %o, want 700", m)
		}
	})

	t.Run("an existing folder is chmod to 0700", func(t *testing.T) {
		home := t.TempDir()
		if err := os.Chmod(home, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Setenv("HARNESS_HOME", home)
		if _, err := process.UserDir(); err != nil {
			t.Fatalf("UserDir: %v", err)
		}
		if m := mode(t, home); m != 0o700 {
			t.Fatalf("mode = %o, want 700", m)
		}
	})

	t.Run("a relative HARNESS_HOME is refused and creates nothing", func(t *testing.T) {
		cwd := t.TempDir()
		t.Chdir(cwd)
		t.Setenv("HARNESS_HOME", "relative/state")
		if got, err := process.UserDir(); err == nil {
			t.Fatalf("UserDir = %q, want an error for a relative path", got)
		}
		if _, err := os.Stat(filepath.Join(cwd, "relative")); !os.IsNotExist(err) {
			t.Fatalf("relative folder was created: %v", err)
		}
	})

	t.Run("no HARNESS_HOME falls back to ~/.harness", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HARNESS_HOME", "")
		t.Setenv("HOME", home)
		got, err := process.UserDir()
		if err != nil {
			t.Fatalf("UserDir: %v", err)
		}
		want := filepath.Join(home, ".harness")
		if got != want {
			t.Fatalf("UserDir = %q, want %q", got, want)
		}
		if m := mode(t, want); m != 0o700 {
			t.Fatalf("mode = %o, want 700", m)
		}
	})
}
