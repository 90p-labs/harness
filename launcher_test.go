package harness

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const (
	launcherPath   = "bin/harness"
	harnessConfig  = ".harness.yaml"
	skippedHook    = "{}"
	exitNoCommand  = 127
	fakeBinary     = "#!/bin/sh\necho \"ran $*\"\n"
	failingGh      = "#!/bin/sh\necho call >>%q\necho %q >&2\nexit 1\n"
	ghError        = "HTTP 404: Not Found"
	executableMode = 0o755
	downloadWait   = 5 * time.Second
	pollInterval   = 50 * time.Millisecond
)

var stopHook = []string{"hook", "--host", "claude", "stop"}

// The launcher must work with only these tools: no gh and no go.
var launcherTools = []string{"sh", "git", "uname", "tr", "mkdir", "find", "rm", "mktemp", "chmod", "mv", "readlink", "cat"}

type launcher struct {
	root, binary, path string
}

func newLauncher(t *testing.T) launcher {
	t.Helper()
	root := t.TempDir()
	for _, file := range []string{launcherPath, claudeManifest} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(root, file)
		if err := os.MkdirAll(filepath.Dir(target), executableMode); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, executableMode); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(root, "dist", "harness-"+runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.MkdirAll(filepath.Dir(binary), executableMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte(fakeBinary), executableMode); err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	for _, tool := range launcherTools {
		found, err := exec.LookPath(tool)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(found, filepath.Join(tools, tool)); err != nil {
			t.Fatal(err)
		}
	}
	return launcher{root: root, binary: binary, path: tools}
}

func (l launcher) run(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(l.root, launcherPath), args...)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + l.path, "HOME=" + t.TempDir()}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), out.String(), errOut.String()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0, out.String(), errOut.String()
}

func gitRepo(t *testing.T, configured bool) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "--quiet", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	if configured {
		if err := os.WriteFile(filepath.Join(dir, harnessConfig), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLauncher(t *testing.T) {
	t.Run("a hook in a subfolder of a configured repository runs the binary", func(t *testing.T) {
		l := newLauncher(t)
		sub := filepath.Join(gitRepo(t, true), "src")
		if err := os.Mkdir(sub, executableMode); err != nil {
			t.Fatal(err)
		}

		code, stdout, stderr := l.run(t, sub, stopHook...)

		if want := "ran " + strings.Join(stopHook, " ") + "\n"; code != 0 || stdout != want {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0 and %q", code, stdout, stderr, want)
		}
	})

	skips := []struct {
		name string
		dir  func(t *testing.T) string
	}{
		{"a hook outside git", func(t *testing.T) string { return t.TempDir() }},
		{"a hook in a repository with no .harness.yaml", func(t *testing.T) string { return gitRepo(t, false) }},
	}
	for _, tc := range skips {
		t.Run(tc.name+" prints an empty JSON object", func(t *testing.T) {
			l := newLauncher(t)

			code, stdout, stderr := l.run(t, tc.dir(t), stopHook...)

			if code != 0 || strings.TrimSpace(stdout) != skippedHook || stderr != "" {
				t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0, %s and no stderr", code, stdout, stderr, skippedHook)
			}
		})
	}

	t.Run("a hook with no binary and no gh prints an empty JSON object and the fix", func(t *testing.T) {
		l := newLauncher(t)
		if err := os.Remove(l.binary); err != nil {
			t.Fatal(err)
		}

		code, stdout, stderr := l.run(t, gitRepo(t, true), stopHook...)

		if code != 0 || strings.TrimSpace(stdout) != skippedHook || !strings.Contains(stderr, "gh auth login") {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0, %s and the fix", code, stdout, stderr, skippedHook)
		}
	})

	t.Run("a hook after a failed download prints the gh error and does not start gh again", func(t *testing.T) {
		l := newLauncher(t)
		if err := os.Remove(l.binary); err != nil {
			t.Fatal(err)
		}
		calls := filepath.Join(t.TempDir(), "gh.log")
		gh := fmt.Sprintf(failingGh, calls, ghError)
		if err := os.WriteFile(filepath.Join(l.path, "gh"), []byte(gh), executableMode); err != nil {
			t.Fatal(err)
		}
		repo := gitRepo(t, true)

		code, stdout, stderr := l.run(t, repo, stopHook...)
		for deadline := time.Now().Add(downloadWait); !strings.Contains(stderr, ghError) && time.Now().Before(deadline); time.Sleep(pollInterval) {
			code, stdout, stderr = l.run(t, repo, stopHook...)
		}

		raw, err := os.ReadFile(calls)
		if err != nil {
			t.Fatal(err)
		}
		if ghCalls := strings.Count(string(raw), "\n"); code != 0 || strings.TrimSpace(stdout) != skippedHook || !strings.Contains(stderr, ghError) || ghCalls != 1 {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, gh calls = %d, want 0, %s, the gh error and 1 gh call", code, stdout, stderr, ghCalls, skippedHook)
		}
	})

	t.Run("a command outside git runs the binary", func(t *testing.T) {
		l := newLauncher(t)

		code, stdout, stderr := l.run(t, t.TempDir(), "version")

		if code != 0 || stdout != "ran version\n" {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0 and the binary output", code, stdout, stderr)
		}
	})

	t.Run("a command with no binary, gh or go exits 127 with the fix", func(t *testing.T) {
		l := newLauncher(t)
		if err := os.Remove(l.binary); err != nil {
			t.Fatal(err)
		}

		code, _, stderr := l.run(t, t.TempDir(), "version")

		if code != exitNoCommand || !strings.Contains(stderr, "gh auth login") {
			t.Fatalf("code = %d, stderr = %q, want %d and the fix", code, stderr, exitNoCommand)
		}
	})
}
