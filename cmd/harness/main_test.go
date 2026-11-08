package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testBuildVersion   = "1.2.3-test"
	envOTLPEndpoint    = "OTEL_EXPORTER_OTLP_ENDPOINT"
	envOTLPTraces      = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	envOTLPMetrics     = "OTEL_EXPORTER_OTLP_METRICS_ENDPOINT"
	invalidOTLPAddress = "localhost:4318"
	telemetryPrefix    = "telemetry: "
	harnessEnvPrefix   = "HARNESS_"
)

func clearOTLPEnv(t *testing.T) {
	t.Helper()
	for _, name := range []string{envOTLPEndpoint, envOTLPTraces, envOTLPMetrics} {
		t.Setenv(name, "")
	}
}

func execute(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	previous := version
	version = testBuildVersion
	t.Cleanup(func() { version = previous })

	var out, errOut bytes.Buffer
	code = run(context.Background(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestVersion(t *testing.T) {
	clearOTLPEnv(t)
	code, stdout, stderr := execute(t, "version")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stdout != "harness 1.2.3-test\n" {
		t.Errorf("stdout = %q, want %q", stdout, "harness 1.2.3-test\n")
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestUsageError(t *testing.T) {
	processArgs := os.Args
	os.Args = []string{processArgs[0], "stray"}
	t.Cleanup(func() { os.Args = processArgs })

	cases := []struct {
		name      string
		args      []string
		wantError string
	}{
		{name: "no command", args: nil, wantError: "error: no command given\n"},
		{name: "unknown command", args: []string{"nope"}, wantError: "error: unknown command \"nope\" for \"harness\"\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearOTLPEnv(t)
			code, stdout, stderr := execute(t, tc.args...)

			if code != 2 {
				t.Errorf("exit code = %d, want 2", code)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.HasPrefix(stderr, tc.wantError) {
				t.Errorf("stderr = %q, want prefix %q", stderr, tc.wantError)
			}
			for _, want := range []string{"Usage:", "harness [command]", "version"} {
				if !strings.Contains(stderr, want) {
					t.Errorf("stderr = %q, want it to contain %q", stderr, want)
				}
			}
		})
	}
}

func clearHarnessEnv(t *testing.T) {
	t.Helper()
	clearOTLPEnv(t)
	for _, entry := range os.Environ() {
		if name, _, _ := strings.Cut(entry, "="); strings.HasPrefix(name, harnessEnvPrefix) {
			t.Setenv(name, "")
		}
	}
}

func TestExitCodes(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		outside   bool
		wantCode  int
		wantError string
	}{
		{name: "unknown flag", args: []string{commandVersion, "--nope"}, wantCode: exitUsage, wantError: "error: unknown flag: --nope\n"},
		{name: "extra argument", args: []string{commandVersion, "extra"}, wantCode: exitUsage, wantError: "error: "},
		{name: "no completion command", args: []string{"completion", "bash", "extra"}, wantCode: exitUsage, wantError: "error: unknown command \"completion\""},
		{name: "outside a git repo", args: []string{commandConfig}, outside: true, wantCode: exitRuntime, wantError: "error: not in a git repository\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			clearHarnessEnv(t)
			if tc.outside {
				t.Chdir(t.TempDir())
			}
			code, _, stderr := execute(t, tc.args...)

			if code != tc.wantCode || !strings.HasPrefix(stderr, tc.wantError) {
				t.Fatalf("code = %d, stderr = %q, want %d and prefix %q", code, stderr, tc.wantCode, tc.wantError)
			}
			if hasUsage := strings.Contains(stderr, "Usage:"); hasUsage != (tc.wantCode == exitUsage) {
				t.Fatalf("stderr = %q, usage shown = %v", stderr, hasUsage)
			}
		})
	}
}

func TestConfigCommand(t *testing.T) {
	clearHarnessEnv(t)
	file := filepath.Join(t.TempDir(), "harness.yaml")
	body := "plugins:\n  - name: guard\n    command: [guard]\n    events: [tool.before]\n  - name: slow\n    command: [slow]\n    events: [tool.after]\n    timeout: 750ms\n"
	if err := os.WriteFile(file, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := execute(t, commandConfig, "--config", file)

	if code != exitOK || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
	for _, want := range []string{"dsn: sqlite:.harness/state.db", "max_review_rounds: 3", "name: guard", "timeout: 5s", "timeout: 750ms", "worker:", "claude: sonnet"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

func TestConfigFlagMissingFile(t *testing.T) {
	clearHarnessEnv(t)
	missing := filepath.Join(t.TempDir(), "missing.yaml")

	code, _, stderr := execute(t, commandConfig, "--config", missing)

	if code != exitRuntime || !strings.Contains(stderr, missing) {
		t.Fatalf("code = %d, stderr = %q, want %d and the file name", code, stderr, exitRuntime)
	}
}

func TestTelemetryErrorKeepsExitCode(t *testing.T) {
	clearOTLPEnv(t)
	t.Setenv(envOTLPEndpoint, invalidOTLPAddress)
	code, stdout, stderr := execute(t, "version")

	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if stdout != "harness 1.2.3-test\n" {
		t.Errorf("stdout = %q, want %q", stdout, "harness 1.2.3-test\n")
	}
	lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
	if len(lines) != 1 || !strings.HasPrefix(lines[0], telemetryPrefix) {
		t.Errorf("stderr = %q, want one line starting with %q", stderr, telemetryPrefix)
	}
}
