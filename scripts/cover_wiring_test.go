package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
	"gopkg.in/yaml.v3"
)

// recipe returns the tab-indented lines of a Makefile target. It reads past a blank line when a comment line follows.
func recipe(t *testing.T, makefile, target string) string {
	t.Helper()
	re := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(target) + `:[^\n]*\n((?:\t[^\n]*\n|\n[ \t]*#[^\n]*\n)*)`)
	m := re.FindStringSubmatch(makefile)
	if m == nil {
		t.Fatalf("Makefile has no %s target", target)
	}
	var kept []string
	for _, line := range strings.SplitAfter(m[1], "\n") {
		if commentLine.MatchString(line) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "")
}

// commentLine matches a line that holds only a comment, after optional spaces or a tab.
var commentLine = regexp.MustCompile(`^[ \t]*#`)

// requireMake fails the test when make is not on PATH.
func requireMake(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("make"); err != nil {
		t.Fatalf("make is not on PATH: it is required, because make check is the CI entry point: %v", err)
	}
}

// makeEnv returns the process environment without the variables that change how make runs.
// Each override is a NAME=value pair that replaces the variable of the same name.
func makeEnv(overrides ...string) []string {
	// GOFLAGS is stripped for every make run, not only the stub subtest: an inherited GOFLAGS is never part of what the Makefile means,
	// and no make subtest should depend on the developer's shell. The stub subtest then proves the Makefile itself sets none.
	strip := []string{"COVERAGE_FLOOR=", "GOFLAGS=", "MAKEFLAGS=", "MAKELEVEL=", "MFLAGS=", "GNUMAKEFLAGS="}
	for _, kv := range overrides {
		name, _, _ := strings.Cut(kv, "=")
		strip = append(strip, name+"=")
	}
	env := append([]string(nil), overrides...)
	for _, kv := range os.Environ() {
		stripped := false
		for _, prefix := range strip {
			if strings.HasPrefix(kv, prefix) {
				stripped = true
				break
			}
		}
		if !stripped {
			env = append(env, kv)
		}
	}
	return env
}

func TestCoverWiring(t *testing.T) {
	makefile := repository.Read(t, "Makefile")

	t.Run("cover is phony", func(t *testing.T) {
		if !regexp.MustCompile(`(?m)^\.PHONY:.*\bcover\b`).MatchString(makefile) {
			t.Error("cover is not listed in .PHONY")
		}
	})

	t.Run("the floor defaults to 85", func(t *testing.T) {
		assign := regexp.MustCompile(`^\s*(?:(?:override|export)\s+)*COVERAGE_FLOOR\s*(?:::=|:=|\?=|\+=|!=|=)`)
		var lines []string
		for _, line := range strings.Split(makefile, "\n") {
			if commentLine.MatchString(line) {
				continue
			}
			if assign.MatchString(line) {
				lines = append(lines, line)
			}
		}
		if len(lines) != 1 {
			t.Fatalf("the Makefile must assign COVERAGE_FLOOR on exactly one line, found %d: %q", len(lines), lines)
		}
		if lines[0] != "COVERAGE_FLOOR ?= 85" {
			t.Errorf("the Makefile must declare `COVERAGE_FLOOR ?= 85`: the floor must be 85, found %q", lines[0])
		}
	})

	t.Run("the effective floor is 85", func(t *testing.T) {
		requireMake(t)
		dir := filepath.Dir(repository.Path(t, "Makefile"))
		// cover is the recipe; check is the local entry point, and an exported target-specific variable on check reaches its nested make cover only.
		// floor checks the merged profile of the CI test shards.
		for _, target := range []string{"cover", "check", "floor"} {
			cmd := exec.Command("make", "-C", dir, "-n", target, "PROFILE=merged.out")
			cmd.Env = makeEnv()
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("make -n %s: %v\n%s", target, err, out)
			}
			var floorLine string
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "scripts/coverfloor.sh") {
					floorLine = line
				}
			}
			if floorLine == "" {
				t.Fatalf("make -n %s has no coverfloor.sh line:\n%s", target, out)
			}
			if !strings.HasSuffix(strings.TrimSpace(floorLine), `"85"`) {
				t.Errorf("the effective floor of make %s must be 85: the coverfloor.sh line must end with %q, found %q", target, `"85"`, floorLine)
			}
		}
	})

	t.Run("check runs cover", func(t *testing.T) {
		body := recipe(t, makefile, "check")
		if !regexp.MustCompile(`(?m)^\t\$\(MAKE\) cover$`).MatchString(body) {
			t.Error("the check recipe does not run $(MAKE) cover")
		}
		if regexp.MustCompile(`\bgo test\b`).MatchString(body) {
			t.Error("the check recipe still holds a plain go test")
		}
	})

	t.Run("cover writes a profile and checks the floor", func(t *testing.T) {
		for _, problem := range coverRecipeProblems(t, makefile) {
			t.Error(problem)
		}
	})

	t.Run("a coverage below the floor fails make cover and make check", func(t *testing.T) {
		requireMake(t)
		dir := filepath.Dir(repository.Path(t, "Makefile"))
		root, err := filepath.EvalSymlinks(dir)
		if err != nil {
			t.Fatalf("resolve the repository root: %v", err)
		}
		// The stub go writes a profile at 0% for go test and does nothing for any other subcommand, so no real test runs.
		stubDir := t.TempDir()
		stub := `#!/bin/sh
if [ "$1" = test ]; then
	if [ -n "$GO_TEST_ARGS_FILE" ]; then
		printf '%s\n' "$@" >> "$GO_TEST_ARGS_FILE"
		printf 'GOFLAGS=%s\n' "$GOFLAGS" >> "$GO_TEST_ARGS_FILE"
		printf 'PWD=%s\n' "$(pwd -P)" >> "$GO_TEST_ARGS_FILE"
		printf '%s\n' -- >> "$GO_TEST_ARGS_FILE"
	fi
	for arg in "$@"; do
		case $arg in
		-coverprofile=*)
			printf 'mode: set\nexample.com/x/x.go:1.1,2.2 1 0\n' > "${arg#-coverprofile=}"
			;;
		esac
	done
fi
exit 0
`
		if err := os.WriteFile(filepath.Join(stubDir, "go"), []byte(stub), 0o755); err != nil {
			t.Fatalf("write the stub go: %v", err)
		}
		for _, target := range []string{"cover", "check"} {
			t.Run(target, func(t *testing.T) {
				cmd := exec.Command("make", "-C", dir, target)
				argsFile := filepath.Join(t.TempDir(), "go-test-args")
				cmd.Env = makeEnv(
					"PATH="+stubDir+string(os.PathListSeparator)+os.Getenv("PATH"),
					"GO_TEST_ARGS_FILE="+argsFile,
				)
				out, err := cmd.CombinedOutput()
				if err == nil {
					t.Errorf("make %s exited 0 with a coverage of 0%%, it must fail below the floor:\n%s", target, out)
				}
				if !strings.Contains(string(out), coverFailure) {
					t.Errorf("make %s output has no coverfloor %q message:\n%s", target, coverFailure, out)
				}
				// The recipe must test the whole module: exactly one go test, with the profile flag and ./... and nothing else,
				// with no GOFLAGS (which can narrow the set with -run or -coverpkg) and from the repository root.
				recorded, rerr := os.ReadFile(argsFile)
				if rerr != nil {
					t.Fatalf("make %s never ran go test: %v", target, rerr)
				}
				lines := strings.Split(strings.TrimSuffix(string(recorded), "\n"), "\n")
				if len(lines) != 6 || lines[0] != "test" || !strings.HasPrefix(lines[1], "-coverprofile=") || len(lines[1]) == len("-coverprofile=") || lines[2] != "./..." || lines[5] != "--" {
					t.Errorf("make %s must run go test exactly once as `go test -coverprofile=<file> ./...`, with no other flag or package; recorded (one per line, -- ends a call): %q", target, lines)
					return
				}
				if lines[3] != "GOFLAGS=" {
					t.Errorf("make %s must run go test with GOFLAGS empty, because GOFLAGS can narrow the measured set; recorded %q", target, lines[3])
				}
				if lines[4] != "PWD="+root {
					t.Errorf("make %s must run go test from the repository root %q; recorded %q", target, root, lines[4])
				}
			})
		}
	})
}

// coverRecipeProblems runs the checks of the cover recipe on a Makefile text and returns one message per failed check.
func coverRecipeProblems(t *testing.T, makefile string) []string {
	t.Helper()
	body := recipe(t, makefile, "cover")
	var problems []string
	if !regexp.MustCompile(`go test\b[^\n]*-coverprofile="\$\$profile"`).MatchString(body) {
		problems = append(problems, `the cover recipe does not pass -coverprofile="$$profile" to go test`)
	}
	if !regexp.MustCompile(`scripts/coverfloor\.sh\s+"\$\$profile"\s+"\$\(COVERAGE_FLOOR\)"`).MatchString(body) {
		problems = append(problems, `the cover recipe does not call scripts/coverfloor.sh with "$(COVERAGE_FLOOR)" quoted`)
	}
	if !regexp.MustCompile(`profile=\$\$\(mktemp\)`).MatchString(body) {
		problems = append(problems, "the cover recipe does not assign the profile from $(mktemp)")
	}
	if !regexp.MustCompile(`trap\s+'rm -f "\$\$profile"'\s+EXIT`).MatchString(body) {
		problems = append(problems, "the cover recipe has no trap that removes the profile with rm -f on EXIT")
	}
	for _, sig := range []string{"INT", "TERM"} {
		if !regexp.MustCompile(`trap\s+'[^']*\bexit [0-9]+[^']*'\s+` + sig + `\b`).MatchString(body) {
			problems = append(problems, "the cover recipe has no trap for "+sig+" that exits with a status")
		}
	}
	return problems
}

func TestCoverGate(t *testing.T) {
	var cfg struct {
		Gates []struct {
			ID  string `yaml:"id"`
			Run string `yaml:"run"`
		} `yaml:"gates"`
	}
	if err := yaml.Unmarshal([]byte(repository.Read(t, ".harness.yaml")), &cfg); err != nil {
		t.Fatalf(".harness.yaml does not parse: %v", err)
	}
	for _, g := range cfg.Gates {
		if g.ID == "go-test" {
			if g.Run != "make cover" {
				t.Errorf("go-test run = %q, want %q", g.Run, "make cover")
			}
			return
		}
	}
	t.Error("no gate with id go-test")
}
