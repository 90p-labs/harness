package main

import (
	"bytes"
	"context"
	"os"
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
