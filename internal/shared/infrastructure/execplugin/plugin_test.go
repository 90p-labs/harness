package execplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/execplugin"
	"github.com/jorge07/harness/internal/shared/infrastructure/telemetry"
)

const (
	helperFlag = "GO_WANT_HELPER_PROCESS"
	helperMode = "HELPER_MODE"

	modeVerdict       = "verdict"
	modeEmpty         = "empty"
	modeEchoEnv       = "echo-traceparent"
	modeEchoRunID     = "echo-run-id"
	modeFail          = "fail"
	modeBadJSON       = "bad-json"
	modeHang          = "hang"
	modeGrandchild    = "grandchild"
	modeSleepHolder   = "sleep-holder"
	modeDetached      = "detached"
	modeSleepChild    = "sleep-child"
	modeStubbornChild = "stubborn-child"

	traceparentUnset = "unset"

	pidFileEnv   = "HELPER_PID_FILE"
	childSeconds = "30"
	callTimeout  = 5 * time.Second
	shortLimit   = 200 * time.Millisecond
	graceBound   = 4 * time.Second
	exitPoll     = 5 * time.Second
	pollStep     = 50 * time.Millisecond
)

var childCommands = map[string][]string{
	modeSleepChild:    {"sleep", childSeconds},
	modeStubbornChild: {"sh", "-c", "trap '' TERM; exec sleep " + childSeconds},
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv(helperFlag) != "1" {
		return
	}
	switch os.Getenv(helperMode) {
	case modeVerdict:
		var envelope domain.Envelope
		if err := json.NewDecoder(os.Stdin).Decode(&envelope); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		fmt.Printf(`{"verdict":"deny","reason":"saw %s on %s","context":"ctx"}`, envelope.Event, envelope.Run.ID)
	case modeEchoEnv:
		value, found := os.LookupEnv(telemetry.EnvTraceparent)
		if !found {
			value = traceparentUnset
		}
		fmt.Printf(`{"verdict":"context","context":%q}`, value)
	case modeEchoRunID:
		fmt.Printf(`{"verdict":"context","context":%q}`, os.Getenv(telemetry.EnvRunID))
	case modeFail:
		fmt.Fprint(os.Stderr, "boom")
		os.Exit(3)
	case modeBadJSON:
		fmt.Print("not json")
	case modeHang, modeSleepHolder:
		time.Sleep(time.Minute)
	case modeGrandchild:
		child := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		child.Env = append(os.Environ(), helperMode+"="+modeSleepHolder)
		child.Stdout = os.Stdout
		if err := child.Start(); err != nil {
			os.Exit(4)
		}
		time.Sleep(time.Minute)
	case modeDetached:
		child := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		child.Env = append(os.Environ(), helperMode+"="+modeSleepHolder)
		child.Stdout = os.Stdout
		if err := child.Start(); err != nil {
			os.Exit(4)
		}
		fmt.Print(`{"verdict":"deny","reason":"detached"}`)
	case modeSleepChild, modeStubbornChild:
		argv := childCommands[os.Getenv(helperMode)]
		child := exec.Command(argv[0], argv[1:]...)
		if err := child.Start(); err != nil {
			os.Exit(4)
		}
		pid := fmt.Sprint(child.Process.Pid)
		if err := os.WriteFile(os.Getenv(pidFileEnv), []byte(pid), 0o600); err != nil {
			os.Exit(4)
		}
		time.Sleep(time.Minute)
	}
	os.Exit(0)
}

func newPlugin(t *testing.T, mode string, timeout time.Duration) execplugin.ExecPlugin {
	t.Helper()
	t.Setenv(helperFlag, "1")
	t.Setenv(helperMode, mode)
	return execplugin.ExecPlugin{
		PluginName: "helper",
		Events:     []domain.EventName{domain.EventToolBefore},
		Command:    []string{os.Args[0], "-test.run=TestHelperProcess"},
		Timeout:    timeout,
	}
}

func testEnvelope(t *testing.T) domain.Envelope {
	t.Helper()
	return domain.Envelope{
		Version: domain.EnvelopeVersion,
		Event:   domain.EventToolBefore,
		Host:    domain.HostClaude,
		Cwd:     t.TempDir(),
		Run:     domain.RunRef{ID: "r1", Phase: "working"},
		Payload: json.RawMessage(`{}`),
	}
}

func TestExecPluginVerdict(t *testing.T) {
	plugin := newPlugin(t, modeVerdict, callTimeout)

	if plugin.Name() != "helper" || len(plugin.Subscribes()) != 1 || plugin.Subscribes()[0] != domain.EventToolBefore {
		t.Fatalf("identity: %q %v", plugin.Name(), plugin.Subscribes())
	}

	got, err := plugin.Handle(context.Background(), testEnvelope(t))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	want := domain.Verdict{Kind: domain.VerdictDeny, Reason: "saw tool.before on r1", Context: "ctx"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestExecPluginEmptyStdoutAllows(t *testing.T) {
	plugin := newPlugin(t, modeEmpty, callTimeout)

	got, err := plugin.Handle(context.Background(), testEnvelope(t))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !reflect.DeepEqual(got, domain.Verdict{Kind: domain.VerdictAllow}) {
		t.Fatalf("got %+v, want allow", got)
	}
}

func TestExecPluginFailure(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		timeout time.Duration
		wantErr string
	}{
		{"non-zero exit", modeFail, callTimeout, "exit status 3"},
		{"invalid json", modeBadJSON, callTimeout, "invalid character"},
		{"timeout", modeHang, shortLimit, context.DeadlineExceeded.Error()},
		{"grandchild keeps stdout open", modeGrandchild, shortLimit, context.DeadlineExceeded.Error()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plugin := newPlugin(t, tc.mode, tc.timeout)

			start := time.Now()
			_, err := plugin.Handle(context.Background(), testEnvelope(t))
			elapsed := time.Since(start)

			if !errors.Is(err, domain.ErrPluginFailed) {
				t.Fatalf("err = %v, want ErrPluginFailed", err)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err %q does not hold %q", err, tc.wantErr)
			}
			if elapsed > graceBound {
				t.Fatalf("call held for %s", elapsed)
			}
		})
	}
}

func TestFailureErrorHasNoStderr(t *testing.T) {
	plugin := newPlugin(t, modeFail, callTimeout)

	_, err := plugin.Handle(context.Background(), testEnvelope(t))

	if !errors.Is(err, domain.ErrPluginFailed) {
		t.Fatalf("err = %v, want ErrPluginFailed", err)
	}
	if strings.Contains(err.Error(), "boom") {
		t.Fatalf("err %q repeats the plugin stderr", err)
	}
}

func TestTimeoutStopsProcessGroup(t *testing.T) {
	for name, mode := range map[string]string{
		"a child that stops on SIGTERM": modeSleepChild,
		"a child that ignores SIGTERM":  modeStubbornChild,
	} {
		t.Run(name, func(t *testing.T) {
			pidFile := filepath.Join(t.TempDir(), "child.pid")
			t.Setenv(pidFileEnv, pidFile)
			plugin := newPlugin(t, mode, time.Second)

			_, err := plugin.Handle(context.Background(), testEnvelope(t))

			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err = %v, want deadline exceeded", err)
			}
			raw, readErr := os.ReadFile(pidFile)
			if readErr != nil {
				t.Fatalf("pid file: %v", readErr)
			}
			pid, convErr := strconv.Atoi(string(raw))
			if convErr != nil {
				t.Fatalf("pid %q: %v", raw, convErr)
			}
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			deadline := time.Now().Add(exitPoll)
			for syscall.Kill(pid, 0) != syscall.ESRCH {
				if time.Now().After(deadline) {
					t.Fatalf("child %d survived the timeout", pid)
				}
				time.Sleep(pollStep)
			}
		})
	}
}

func TestExecPluginDetachedChildKeepsVerdict(t *testing.T) {
	plugin := newPlugin(t, modeDetached, callTimeout)

	got, err := plugin.Handle(context.Background(), testEnvelope(t))

	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if !reflect.DeepEqual(got, domain.Verdict{Kind: domain.VerdictDeny, Reason: "detached"}) {
		t.Fatalf("got %+v, want the detached deny", got)
	}
}

func TestExecPluginRelativeCommandRunsInEnvelopeCwd(t *testing.T) {
	worktree := t.TempDir()
	script := "#!/bin/sh\necho '{\"verdict\":\"deny\",\"reason\":\"from the worktree\"}'\n"
	if err := os.WriteFile(filepath.Join(worktree, "gate.sh"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	envelope := testEnvelope(t)
	envelope.Cwd = worktree
	plugin := execplugin.ExecPlugin{PluginName: "gate", Command: []string{"./gate.sh"}, Timeout: callTimeout}

	got, err := plugin.Handle(context.Background(), envelope)

	if err != nil || !reflect.DeepEqual(got, domain.Verdict{Kind: domain.VerdictDeny, Reason: "from the worktree"}) {
		t.Fatalf("got %+v, %v, want the deny of the worktree script", got, err)
	}
}

func TestExecPluginEmptyCommandFails(t *testing.T) {
	_, err := execplugin.ExecPlugin{PluginName: "empty"}.Handle(context.Background(), testEnvelope(t))

	if !errors.Is(err, domain.ErrPluginFailed) {
		t.Fatalf("err = %v, want ErrPluginFailed", err)
	}
}

func TestExecPluginTraceparent(t *testing.T) {
	plugin := newPlugin(t, modeEchoEnv, callTimeout)
	traceID, _ := trace.TraceIDFromHex("0af7651916cd43dd8448eb211c80319c")
	spanID, _ := trace.SpanIDFromHex("b7ad6b7169203331")
	ctx := trace.ContextWithSpanContext(context.Background(), trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	}))

	got, err := plugin.Handle(ctx, testEnvelope(t))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	const want = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if got.Kind != domain.VerdictContext || got.Context != want {
		t.Fatalf("got %+v, want TRACEPARENT %s", got, want)
	}
}

func TestExecPluginRunID(t *testing.T) {
	t.Setenv(telemetry.EnvRunID, "inherited")
	plugin := newPlugin(t, modeEchoRunID, callTimeout)
	ctx := telemetry.WithRun(context.Background(), "", "run-7")

	got, err := plugin.Handle(ctx, testEnvelope(t))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got.Kind != domain.VerdictContext || got.Context != "run-7" {
		t.Fatalf("got %+v, want HARNESS_RUN_ID run-7", got)
	}
}

func TestTraceparentNoSpan(t *testing.T) {
	// Setenv registers the restore at cleanup; Unsetenv then removes the variable.
	t.Setenv(telemetry.EnvTraceparent, "")
	t.Setenv(telemetry.EnvRunID, "")
	if err := os.Unsetenv(telemetry.EnvTraceparent); err != nil {
		t.Fatal(err)
	}
	plugin := newPlugin(t, modeEchoEnv, callTimeout)

	got, err := plugin.Handle(context.Background(), testEnvelope(t))
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got.Kind != domain.VerdictContext || got.Context != traceparentUnset {
		t.Fatalf("got %+v, want TRACEPARENT unset", got)
	}
}
