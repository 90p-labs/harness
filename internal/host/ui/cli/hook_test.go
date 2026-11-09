package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/metric/noop"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	"github.com/90p-labs/harness/internal/host/application/command/handlehook"
	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	"github.com/90p-labs/harness/internal/host/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type denyDispatcher struct{}

func (denyDispatcher) Dispatch(context.Context, shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	return shareddomain.DispatchResult{Verdict: shareddomain.Verdict{Kind: shareddomain.VerdictDeny, Reason: "not now"}}, nil
}

type unreadable struct{ t *testing.T }

func (u unreadable) Read([]byte) (int, error) {
	u.t.Error("stdin was read")
	return 0, errors.New("unreadable")
}

type printingCodec struct{ fakeCodec }

func (printingCodec) Encode(hook hostdomain.HookName, v shareddomain.Verdict) ([]byte, error) {
	return []byte(string(hook) + " " + string(v.Kind) + " " + v.Reason), nil
}

func runHook(t *testing.T, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	handler := handlehook.NewHandler(
		[]hostdomain.Codec{printingCodec{fakeCodec{host: shareddomain.HostClaude, events: hostdomain.AllEvents}}},
		func(context.Context) (shareddomain.Dispatcher, error) { return denyDispatcher{}, nil },
		tracenoop.NewTracerProvider().Tracer("test"),
		noop.NewMeterProvider().Meter("test"),
	)
	cmd := cli.NewHookCommand(handler)
	var out, errOut bytes.Buffer
	cmd.SetIn(strings.NewReader(stdin))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestHookCommand(t *testing.T) {
	const input = `{"cwd":"/work","tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`

	stdout, stderr, err := runHook(t, input, "--host", "claude", "pre-tool-use")

	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if want := "pre-tool-use deny not now"; stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestHookCommandRefusesBeforeReadingStdin(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{name: "unknown host", args: []string{"--host", "vim", "pre-tool-use"}, wantErr: cli.ErrUnknownHost},
		{name: "unknown hook", args: []string{"--host", "claude", "PreToolUse"}, wantErr: hostdomain.ErrUnknownHook},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := cli.NewHookCommand(handlehook.Handler{})
			cmd.SetIn(unreadable{t})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tc.args)

			if err := cmd.Execute(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}
