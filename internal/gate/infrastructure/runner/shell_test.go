package runner_test

import (
	"context"
	"strings"
	"testing"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/gate/infrastructure/runner"
	"github.com/jorge07/harness/internal/shared/infrastructure/telemetry"
)

var _ domain.Runner = runner.Shell{}

func newGate(t *testing.T, run string, timeout time.Duration) domain.Gate {
	t.Helper()
	gate, err := domain.NewGate("probe", run, nil, timeout, "")
	if err != nil {
		t.Fatal(err)
	}
	return gate
}

func TestShellRunnerPass(t *testing.T) {
	out, err := runner.Shell{}.Run(context.Background(), t.TempDir(), newGate(t, "echo out; echo err >&2", time.Minute))
	if err != nil || out.Result != domain.ResultPass {
		t.Fatalf("Run = %+v, %v, want pass", out, err)
	}
	if out.Stdout != "out\n" {
		t.Fatalf("Stdout = %q, want only standard output", out.Stdout)
	}
	if !strings.Contains(out.Tail, "out") || !strings.Contains(out.Tail, "err") {
		t.Fatalf("Tail = %q, want both streams", out.Tail)
	}
}

func TestShellRunnerFail(t *testing.T) {
	out, err := runner.Shell{}.Run(context.Background(), t.TempDir(), newGate(t, "echo boom; exit 3", time.Minute))
	if err != nil || out.Result != domain.ResultFail || out.TimedOut || !strings.Contains(out.Tail, "boom") {
		t.Fatalf("Run = %+v, %v, want fail with the output", out, err)
	}
}

func TestShellRunnerRunsInDir(t *testing.T) {
	dir := t.TempDir()
	out, err := runner.Shell{}.Run(context.Background(), dir, newGate(t, "pwd -P", time.Minute))
	if err != nil || !strings.HasSuffix(strings.TrimSpace(out.Stdout), strings.TrimPrefix(dir, "/private")) {
		t.Fatalf("Stdout = %q, %v, want the dir %q", out.Stdout, err, dir)
	}
}

func TestShellRunnerKeepsLastFourKiB(t *testing.T) {
	const tail = 4096
	run := "i=0; while [ $i -lt 1000 ]; do printf '%08d\\n' $i; i=$((i+1)); done"
	out, err := runner.Shell{}.Run(context.Background(), t.TempDir(), newGate(t, run, time.Minute))
	if err != nil || out.Result != domain.ResultPass {
		t.Fatalf("Run = %v, %v", out.Result, err)
	}
	if len(out.Tail) != tail {
		t.Fatalf("len(Tail) = %d, want %d", len(out.Tail), tail)
	}
	if !strings.HasSuffix(out.Tail, "00000999\n") {
		t.Fatalf("Tail does not end with the last line: %q", out.Tail[len(out.Tail)-20:])
	}
	if len(out.Stdout) != 9000 {
		t.Fatalf("len(Stdout) = %d, want the full 9000 bytes", len(out.Stdout))
	}
}

func TestShellRunnerTimeout(t *testing.T) {
	start := time.Now()
	out, err := runner.Shell{}.Run(context.Background(), t.TempDir(), newGate(t, "sleep 5", 200*time.Millisecond))
	if err != nil || out.Result != domain.ResultFail || !out.TimedOut {
		t.Fatalf("Run = %+v, %v, want a timed out fail", out, err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout took %v, want the child killed", elapsed)
	}
}

func TestShellRunnerTimeoutKillsChildren(t *testing.T) {
	start := time.Now()
	out, err := runner.Shell{}.Run(context.Background(), t.TempDir(), newGate(t, "sleep 5 | cat", 200*time.Millisecond))
	if err != nil || out.Result != domain.ResultFail {
		t.Fatalf("Run = %+v, %v, want fail", out, err)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Fatalf("timeout took %v, want the process group killed", elapsed)
	}
}

const (
	printTraceparent = "printenv " + telemetry.EnvTraceparent
	inheritedParent  = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
)

func TestTraceparent(t *testing.T) {
	t.Setenv(telemetry.EnvTraceparent, inheritedParent)

	t.Run("span in context", func(t *testing.T) {
		ctx, span := sdktrace.NewTracerProvider().Tracer("test").Start(context.Background(), "gate")
		defer span.End()
		out, err := runner.Shell{}.Run(ctx, t.TempDir(), newGate(t, printTraceparent, time.Minute))
		if err != nil {
			t.Fatal(err)
		}
		want := span.SpanContext().TraceID().String()
		if !strings.Contains(out.Stdout, want) || strings.Contains(out.Stdout, inheritedParent) {
			t.Fatalf("TRACEPARENT = %q, want trace id %s", out.Stdout, want)
		}
	})

	t.Run("no span in context", func(t *testing.T) {
		out, err := runner.Shell{}.Run(context.Background(), t.TempDir(), newGate(t, printTraceparent, time.Minute))
		if err != nil || strings.TrimSpace(out.Stdout) != inheritedParent {
			t.Fatalf("TRACEPARENT = %q, %v, want the inherited value", out.Stdout, err)
		}
	})
}
