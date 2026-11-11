package scripts_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	coverFloorScript = "scripts/coverfloor.sh"
	coverFailure     = "below the floor"
	coverFileMode    = 0o644
	coverExitRefused = 2
)

// runCoverFloor runs the script and returns stdout, stderr and the exit code.
func runCoverFloor(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command("sh", append([]string{repository.Path(t, coverFloorScript)}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run %v: %v", args, err)
		}
		code = exit.ExitCode()
	}
	return stdout.String(), stderr.String(), code
}

func writeProfile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cover.out")
	if err := os.WriteFile(path, []byte(body), coverFileMode); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCoverFloor(t *testing.T) {
	cases := []struct {
		name    string
		profile string
		floor   string
		code    int
		line    string
	}{
		{
			name:    "low total fails",
			profile: "mode: set\na.go:1.1,2.1 95 1\na.go:3.1,4.1 905 0\n",
			floor:   "85",
			code:    1,
			line:    "coverage: 9.5% (floor 85%)",
		},
		{
			name:    "full coverage passes",
			profile: "mode: count\na.go:1.1,2.1 40 3\nb.go:1.1,2.1 60 1\n",
			floor:   "85",
			code:    0,
			line:    "coverage: 100.0% (floor 85%)",
		},
		{
			name:    "exactly the floor passes",
			profile: "mode: atomic\na.go:1.1,2.1 85 2\na.go:3.1,4.1 15 0\n",
			floor:   "85.0",
			code:    0,
			line:    "coverage: 85.0% (floor 85.0%)",
		},
		{
			name:    "just under the floor fails",
			profile: "mode: set\na.go:1.1,2.1 8496 1\na.go:3.1,4.1 1504 0\n",
			floor:   "85",
			code:    1,
			line:    "coverage: 84.9% (floor 85%)",
		},
		{
			name: "a duplicated block counts once and covered if any copy is hit",
			profile: "mode: set\n" +
				"a.go:1.1,2.1 50 0\n" +
				"a.go:1.1,2.1 50 1\n" +
				"b.go:1.1,2.1 50 0\n" +
				"b.go:1.1,2.1 50 0\n",
			floor: "50",
			code:  0,
			line:  "coverage: 50.0% (floor 50%)",
		},
		{
			name: "a duplicated block does not double the statements",
			profile: "mode: set\n" +
				"a.go:1.1,2.1 50 1\n" +
				"a.go:1.1,2.1 50 1\n" +
				"b.go:1.1,2.1 50 0\n",
			floor: "67",
			code:  1,
			line:  "coverage: 50.0% (floor 67%)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runCoverFloor(t, writeProfile(t, tc.profile), tc.floor)
			if code != tc.code {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", code, tc.code, stdout, stderr)
			}
			if strings.TrimSpace(stdout) != tc.line {
				t.Errorf("stdout = %q, want %q", stdout, tc.line)
			}
			if tc.code == 1 && !strings.Contains(stderr, coverFailure) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, coverFailure)
			}
			if tc.code == 0 && stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

func TestCoverFloorRefuses(t *testing.T) {
	good := writeProfile(t, "mode: set\na.go:1.1,2.1 10 1\n")
	modeOnly := writeProfile(t, "mode: set\n")
	empty := writeProfile(t, "")
	shortLine := writeProfile(t, "mode: set\na.go:1.1,2.1 10 1\na.go:3.1,4.1 10\n")
	longLine := writeProfile(t, "mode: set\na.go:1.1,2.1 10 1\na.go:3.1,4.1 10 1 extra\n")
	badStmtsText := writeProfile(t, "mode: set\na.go:1.1,2.1 10 1\nb.go:1.1,2.1 abc 1\n")
	badHitsText := writeProfile(t, "mode: set\na.go:1.1,2.1 10 1\nb.go:1.1,2.1 10 xyz\n")
	badStmtsNegative := writeProfile(t, "mode: set\na.go:1.1,2.1 10 1\nb.go:1.1,2.1 -50 0\n")
	badHitsNegative := writeProfile(t, "mode: set\na.go:1.1,2.1 10 1\nb.go:1.1,2.1 10 -1\n")
	noMode := writeProfile(t, "a.go:1.1,2.1 10 1\n")
	cases := []struct {
		name   string
		args   []string
		stderr string
	}{
		{"non-numeric statement count", []string{badStmtsText, "50"}, "line 3 of the profile"},
		{"non-numeric hit count", []string{badHitsText, "50"}, "line 3 of the profile"},
		{"negative statement count", []string{badStmtsNegative, "50"}, "line 3 of the profile"},
		{"negative hit count", []string{badHitsNegative, "50"}, "line 3 of the profile"},
		{"empty floor", []string{good, ""}, "the floor must be"},
		{"letters as floor", []string{good, "abc"}, "the floor must be"},
		{"negative floor", []string{good, "-5"}, "the floor must be"},
		{"trailing dot floor", []string{good, "85."}, "the floor must be"},
		{"leading dot floor", []string{good, ".5"}, "the floor must be"},
		{"two dots floor", []string{good, "1.2.3"}, "the floor must be"},
		{"missing file", []string{filepath.Join(t.TempDir(), "absent.out"), "85"}, "cannot read the profile"},
		{"mode only profile", []string{modeOnly, "85"}, "no statements"},
		{"empty profile", []string{empty, "85"}, "no statements"},
		{"profile without a mode line", []string{noMode, "50"}, "mode line"},
		{"floor with an embedded newline", []string{good, "85\nabc"}, "the floor must be"},
		{"profile line without three fields", []string{shortLine, "50"}, "three fields"},
		{"profile line with four fields", []string{longLine, "50"}, "three fields"},
		{"no arguments", nil, "usage"},
		{"one argument", []string{good}, "usage"},
		{"three arguments", []string{good, "85", "extra"}, "usage"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runCoverFloor(t, tc.args...)
			if code != coverExitRefused {
				t.Fatalf("exit = %d, want %d\nstdout: %s\nstderr: %s", code, coverExitRefused, stdout, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, tc.stderr) {
				t.Errorf("stderr = %q, want it to contain %q", stderr, tc.stderr)
			}
		})
	}
}
