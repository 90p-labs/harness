package execplugin_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/jorge07/harness/internal/plugin/domain"
	"github.com/jorge07/harness/internal/plugin/infrastructure/execplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	helperFlag = "GO_WANT_HELPER_PROCESS"
	helperMode = "HELPER_MODE"

	modeVerdict     = "verdict"
	modeEmpty       = "empty"
	modeEchoEnv     = "echo-traceparent"
	modeFail        = "fail"
	modeBadJSON     = "bad-json"
	modeHang        = "hang"
	modeGrandchild  = "grandchild"
	modeSleepHolder = "sleep-holder"
	modeNoisyFail   = "noisy-fail"
	modeDetached    = "detached"

	stderrTail  = 512
	callTimeout = 5 * time.Second
	shortLimit  = 200 * time.Millisecond
)

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
		fmt.Printf(`{"verdict":"context","context":%q}`, os.Getenv("TRACEPARENT"))
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
	case modeNoisyFail:
		fmt.Fprint(os.Stderr, strings.Repeat("a", 2*stderrTail)+"END")
		os.Exit(5)
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

func testEnvelope() domain.Envelope {
	return domain.Envelope{
		Version: domain.EnvelopeVersion,
		Event:   domain.EventToolBefore,
		Host:    shareddomain.HostClaude,
		Cwd:     "/repo",
		Run:     domain.RunRef{ID: "r1", Phase: "working"},
		Payload: json.RawMessage(`{}`),
	}
}

func TestExecPluginVerdict(t *testing.T) {
	plugin := newPlugin(t, modeVerdict, callTimeout)

	if plugin.Name() != "helper" || len(plugin.Subscribes()) != 1 || plugin.Subscribes()[0] != domain.EventToolBefore {
		t.Fatalf("identity: %q %v", plugin.Name(), plugin.Subscribes())
	}

	got, err := plugin.Handle(context.Background(), testEnvelope())
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	want := domain.Verdict{Kind: domain.VerdictDeny, Reason: "saw tool.before on r1", Context: "ctx"}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestExecPluginEmptyStdoutAllows(t *testing.T) {
	plugin := newPlugin(t, modeEmpty, callTimeout)

	got, err := plugin.Handle(context.Background(), testEnvelope())
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got != (domain.Verdict{Kind: domain.VerdictAllow}) {
		t.Fatalf("got %+v, want allow", got)
	}
}

func TestExecPluginFailure(t *testing.T) {
	cases := []struct {
		name    string
		mode    string
		timeout time.Duration
		stderr  string
	}{
		{"non-zero exit", modeFail, callTimeout, "boom"},
		{"invalid json", modeBadJSON, callTimeout, ""},
		{"timeout", modeHang, shortLimit, ""},
		{"grandchild keeps stdout open", modeGrandchild, shortLimit, ""},
		{"stderr cut to the last bytes", modeNoisyFail, callTimeout, strings.Repeat("a", stderrTail-3) + "END"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plugin := newPlugin(t, tc.mode, tc.timeout)

			start := time.Now()
			_, err := plugin.Handle(context.Background(), testEnvelope())
			elapsed := time.Since(start)

			if !errors.Is(err, domain.ErrPluginFailed) {
				t.Fatalf("err = %v, want ErrPluginFailed", err)
			}
			if !strings.Contains(err.Error(), tc.stderr) {
				t.Fatalf("err %q does not hold stderr %q", err, tc.stderr)
			}
			if tc.mode == modeNoisyFail && strings.Contains(err.Error(), strings.Repeat("a", stderrTail)) {
				t.Fatalf("stderr not cut to %d bytes", stderrTail)
			}
			if elapsed > callTimeout {
				t.Fatalf("call held for %s", elapsed)
			}
		})
	}
}

func TestExecPluginDetachedChildKeepsVerdict(t *testing.T) {
	plugin := newPlugin(t, modeDetached, callTimeout)

	got, err := plugin.Handle(context.Background(), testEnvelope())

	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	if got != (domain.Verdict{Kind: domain.VerdictDeny, Reason: "detached"}) {
		t.Fatalf("got %+v, want the detached deny", got)
	}
}

func TestExecPluginEmptyCommandFails(t *testing.T) {
	_, err := execplugin.ExecPlugin{PluginName: "empty"}.Handle(context.Background(), testEnvelope())

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

	got, err := plugin.Handle(ctx, testEnvelope())
	if err != nil {
		t.Fatalf("handle: %v", err)
	}
	const want = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"
	if got.Kind != domain.VerdictContext || got.Context != want {
		t.Fatalf("got %+v, want TRACEPARENT %s", got, want)
	}
}
