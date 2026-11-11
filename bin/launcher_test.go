package bin_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	launcherPath  = "bin/harness"
	harnessConfig = ".harness.yaml"
	skippedHook   = "{}"
	exitNoCommand = 127
	fakeBinary    = "#!/bin/sh\necho \"ran $*\"\n"
	failingFetch  = "#!/bin/sh\necho call >>%q\necho %q >&2\nexit 1\n"
	// A fetch tool that logs its arguments and writes the binary to the path after -o or -O.
	workingFetch    = "#!/bin/sh\necho \"$*\" >>%q\nwhile [ $# -gt 0 ]; do case $1 in -o|-O) out=$2; shift ;; esac; shift; done\nprintf '#!/bin/sh\\necho \"downloaded $*\"\\n' >\"$out\"\n"
	loggingFind     = "#!/bin/sh\n%q \"$@\"\nstatus=$?\necho done >>%q\nexit $status\n"
	fetchError      = "curl: (22) The requested URL returned error: 404"
	executableMode  = 0o755
	privateMode     = 0o600
	downloadLock    = ".download.lock"
	downloadFailure = ".download.failed"
	downloadWait    = 30 * time.Second
	pollInterval    = 50 * time.Millisecond
	hookReachesLock = 200 * time.Millisecond
	claudeManifest  = ".claude-plugin/plugin.json"
)

var (
	stopHook = []string{"hook", "--host", "claude", "stop"}
	// The launcher must work with only these tools: no curl, no wget and no go.
	launcherTools = []string{"sh", "git", "uname", "tr", "mkdir", "find", "rm", "mktemp", "chmod", "mv", "readlink", "cat"}
)

type launcher struct {
	root, binary, path string
}

func newLauncher(t *testing.T) launcher {
	t.Helper()
	root := t.TempDir()
	for _, file := range []string{launcherPath, claudeManifest} {
		raw, err := os.ReadFile(repository.Path(t, file))
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

// fakeFetch puts a fetch tool on the launcher PATH and returns the file that logs its calls.
func (l launcher) fakeFetch(t *testing.T, tool, script string, extra ...any) string {
	t.Helper()
	calls := filepath.Join(t.TempDir(), tool+".log")
	body := fmt.Sprintf(script, append([]any{calls}, extra...)...)
	if err := os.WriteFile(filepath.Join(l.path, tool), []byte(body), executableMode); err != nil {
		t.Fatal(err)
	}
	return calls
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

	t.Run("a hook with no binary, curl or wget prints an empty JSON object and the fix", func(t *testing.T) {
		l := newLauncher(t)
		if err := os.Remove(l.binary); err != nil {
			t.Fatal(err)
		}

		code, stdout, stderr := l.run(t, gitRepo(t, true), stopHook...)

		if code != 0 || strings.TrimSpace(stdout) != skippedHook || !strings.Contains(stderr, "install curl or wget") {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0, %s and the fix", code, stdout, stderr, skippedHook)
		}
	})

	t.Run("a hook after a failed download prints the curl error and does not start curl again", func(t *testing.T) {
		l := newLauncher(t)
		if err := os.Remove(l.binary); err != nil {
			t.Fatal(err)
		}
		calls := l.fakeFetch(t, "curl", failingFetch, fetchError)
		repo := gitRepo(t, true)

		code, stdout, stderr := l.run(t, repo, stopHook...)
		for deadline := time.Now().Add(downloadWait); !strings.Contains(stderr, fetchError) && time.Now().Before(deadline); time.Sleep(pollInterval) {
			code, stdout, stderr = l.run(t, repo, stopHook...)
		}

		raw, err := os.ReadFile(calls)
		if err != nil {
			t.Fatal(err)
		}
		if fetchCalls := strings.Count(string(raw), "\n"); code != 0 || strings.TrimSpace(stdout) != skippedHook || !strings.Contains(stderr, fetchError) || fetchCalls != 1 {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, curl calls = %d, want 0, %s, the curl error and 1 curl call", code, stdout, stderr, fetchCalls, skippedHook)
		}
	})

	t.Run("a hook that gets the lock after a failed download prints the curl error and does not start curl", func(t *testing.T) {
		l := newLauncher(t)
		if err := os.Remove(l.binary); err != nil {
			t.Fatal(err)
		}
		calls := l.fakeFetch(t, "curl", failingFetch, fetchError)
		dist := filepath.Dir(l.binary)
		lock, failure := filepath.Join(dist, downloadLock), filepath.Join(dist, downloadFailure)
		// The noclobber redirect opens a FIFO and blocks there until a reader opens it.
		if err := syscall.Mkfifo(lock, privateMode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if reader, err := os.OpenFile(lock, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
				_ = reader.Close()
			}
		})
		finds := filepath.Join(t.TempDir(), "find.log")
		realFind, err := os.Readlink(filepath.Join(l.path, "find"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(l.path, "find")); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(l.path, "find"), []byte(fmt.Sprintf(loggingFind, realFind, finds)), executableMode); err != nil {
			t.Fatal(err)
		}
		repo := gitRepo(t, true)
		type result struct {
			code           int
			stdout, stderr string
		}
		done := make(chan result)
		go func() {
			code, stdout, stderr := l.run(t, repo, stopHook...)
			done <- result{code, stdout, stderr}
		}()

		// The stale checks of the lock and the failure file are the last commands before the failure check.
		for deadline := time.Now().Add(downloadWait); ; time.Sleep(pollInterval) {
			if raw, _ := os.ReadFile(finds); strings.Count(string(raw), "\n") == 2 {
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("the hook did not reach the stale checks")
			}
		}

		// The wait lets an early failure check pass before the failure file exists; the fixed hook passes in any order.
		time.Sleep(hookReachesLock)
		if err := os.WriteFile(failure, []byte(fetchError), privateMode); err != nil {
			t.Fatal(err)
		}
		reader, err := os.OpenFile(lock, os.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			t.Fatal(err)
		}
		got := <-done
		if err := reader.Close(); err != nil {
			t.Fatal(err)
		}

		_, fetchErr := os.Stat(calls)
		_, lockErr := os.Lstat(lock)
		if got.code != 0 || strings.TrimSpace(got.stdout) != skippedHook || !strings.Contains(got.stderr, fetchError) || !errors.Is(fetchErr, os.ErrNotExist) || !errors.Is(lockErr, os.ErrNotExist) {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, curl log error = %v, lock error = %v, want 0, %s, the curl error, no curl call and no lock", got.code, got.stdout, got.stderr, fetchErr, lockErr, skippedHook)
		}
	})

	t.Run("a command outside git runs the binary", func(t *testing.T) {
		l := newLauncher(t)

		code, stdout, stderr := l.run(t, t.TempDir(), "version")

		if code != 0 || stdout != "ran version\n" {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0 and the binary output", code, stdout, stderr)
		}
	})

	t.Run("a command with no binary, curl, wget or go exits 127 with the fix", func(t *testing.T) {
		l := newLauncher(t)
		if err := os.Remove(l.binary); err != nil {
			t.Fatal(err)
		}

		code, _, stderr := l.run(t, t.TempDir(), "version")

		if code != exitNoCommand || !strings.Contains(stderr, "install curl or wget") {
			t.Fatalf("code = %d, stderr = %q, want %d and the fix", code, stderr, exitNoCommand)
		}
	})

	for _, tool := range []string{"curl", "wget"} {
		t.Run("a command with no binary downloads the release of the plugin version with "+tool, func(t *testing.T) {
			l := newLauncher(t)
			if err := os.Remove(l.binary); err != nil {
				t.Fatal(err)
			}
			calls := l.fakeFetch(t, tool, workingFetch)

			code, stdout, stderr := l.run(t, t.TempDir(), "version")

			raw, err := os.ReadFile(calls)
			if err != nil {
				t.Fatal(err)
			}
			url := "https://github.com/90p-labs/harness/releases/download/v" + pluginVersion(t) + "/harness-" + runtime.GOOS + "-" + runtime.GOARCH
			if code != 0 || stdout != "downloaded version\n" || !strings.Contains(string(raw), url) {
				t.Fatalf("code = %d, stdout = %q, stderr = %q, %s args = %q, want 0, the downloaded binary output and %s", code, stdout, stderr, tool, raw, url)
			}
		})
	}
}

func pluginVersion(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(repository.Path(t, claudeManifest))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil || manifest.Version == "" {
		t.Fatalf("version of %s: %v", claudeManifest, err)
	}
	return manifest.Version
}
