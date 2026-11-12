package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	"github.com/90p-labs/harness/internal/host/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

type fakeCodec struct {
	host   shareddomain.Host
	events []shareddomain.EventName
}

func (f fakeCodec) Host() shareddomain.Host { return f.host }

func (f fakeCodec) Capabilities() []shareddomain.EventName { return f.events }

func (f fakeCodec) Decode(hostdomain.HookName, []byte) (shareddomain.Envelope, error) {
	return shareddomain.Envelope{}, nil
}

func (f fakeCodec) Encode(hostdomain.HookName, shareddomain.Verdict) ([]byte, error) {
	return nil, nil
}

func run(t *testing.T, handlers cli.HostHandlers, args ...string) (string, error) {
	t.Helper()
	out, _, err := runBoth(t, handlers, args...)
	return out, err
}

func runBoth(t *testing.T, handlers cli.HostHandlers, args ...string) (string, string, error) {
	t.Helper()
	cmd := cli.NewHostCommand(func(context.Context) (cli.HostHandlers, error) { return handlers, nil })
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func noPlugin(context.Context, shareddomain.Host) (bool, error) { return false, nil }

func TestHostPlugin(t *testing.T) {
	const settingsPath = "/work/.claude/settings.json"
	var uninstalled []shareddomain.Host
	handlers := cli.HostHandlers{
		Install: func(context.Context, shareddomain.Host) (string, error) { return settingsPath, nil },
		Plugin:  func(context.Context, shareddomain.Host) (bool, error) { return true, nil },
		Uninstall: func(_ context.Context, host shareddomain.Host) (string, int, error) {
			uninstalled = append(uninstalled, host)
			return settingsPath, 9, nil
		},
	}

	t.Run("install warns that the plugin runs the hooks too", func(t *testing.T) {
		out, errOut, err := runBoth(t, handlers, "install", "--host", "claude")
		if err != nil || strings.TrimSpace(out) != settingsPath {
			t.Fatalf("out = %q, err = %v", out, err)
		}
		want := "warning: the 90p plugin also runs the harness hooks, so each hook runs twice; " +
			"remove them from this file with harness host uninstall --host claude\n"
		if errOut != want {
			t.Fatalf("stderr = %q, want %q", errOut, want)
		}
	})

	t.Run("install without the plugin prints no warning", func(t *testing.T) {
		quiet := handlers
		quiet.Plugin = noPlugin
		if _, errOut, err := runBoth(t, quiet, "install", "--host", "claude"); err != nil || errOut != "" {
			t.Fatalf("stderr = %q, err = %v", errOut, err)
		}
	})

	t.Run("install returns the plugin check error", func(t *testing.T) {
		boom := errors.New("boom")
		failing := handlers
		failing.Plugin = func(context.Context, shareddomain.Host) (bool, error) { return false, boom }
		if _, err := run(t, failing, "install", "--host", "claude"); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
	})

	t.Run("uninstall prints the count and the path", func(t *testing.T) {
		out, err := run(t, handlers, "uninstall")
		if err != nil || out != "removed 9 harness hooks from "+settingsPath+"\n" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
		if len(uninstalled) != 1 || uninstalled[0] != "" {
			t.Fatalf("uninstalled = %q, want one empty host", uninstalled)
		}
	})

	t.Run("uninstall passes the host and returns its error", func(t *testing.T) {
		boom := errors.New("boom")
		failing := handlers
		failing.Uninstall = func(_ context.Context, host shareddomain.Host) (string, int, error) {
			if host != shareddomain.HostCodex {
				t.Errorf("host = %q, want codex", host)
			}
			return "", 0, boom
		}
		if _, err := run(t, failing, "uninstall", "--host", "codex"); !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
		if _, err := run(t, failing, "uninstall", "--host", "vim"); !errors.Is(err, cli.ErrUnknownHost) {
			t.Fatalf("err = %v, want ErrUnknownHost", err)
		}
	})
}

func TestHostCommand(t *testing.T) {
	var installed []shareddomain.Host
	handlers := cli.HostHandlers{
		Install: func(_ context.Context, host shareddomain.Host) (string, error) {
			installed = append(installed, host)
			return "/work/.codex/hooks.json", nil
		},
		Plugin: noPlugin,
		Codecs: []hostdomain.Codec{
			fakeCodec{host: shareddomain.HostClaude, events: hostdomain.AllEvents},
			fakeCodec{host: shareddomain.HostCodex, events: []shareddomain.EventName{
				shareddomain.EventSessionStarted,
				shareddomain.EventPromptSubmitted,
				shareddomain.EventToolBefore,
				shareddomain.EventTurnStopping,
			}},
		},
	}

	t.Run("install prints the written path", func(t *testing.T) {
		out, err := run(t, handlers, "install", "--host", "codex")
		if err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(out) != "/work/.codex/hooks.json" {
			t.Fatalf("output = %q", out)
		}
		if len(installed) != 1 || installed[0] != shareddomain.HostCodex {
			t.Fatalf("installed = %v, want [codex]", installed)
		}
	})

	t.Run("install with no --host asks the handler to detect the host", func(t *testing.T) {
		installed = nil

		if _, err := run(t, handlers, "install"); err != nil {
			t.Fatal(err)
		}
		if len(installed) != 1 || installed[0] != "" {
			t.Fatalf("installed = %q, want one empty host", installed)
		}
	})

	t.Run("install returns the detection refusal", func(t *testing.T) {
		refusing := handlers
		refusing.Install = func(context.Context, shareddomain.Host) (string, error) {
			return "", hostdomain.ErrManyHosts
		}

		if _, err := run(t, refusing, "install"); !errors.Is(err, hostdomain.ErrManyHosts) {
			t.Fatalf("err = %v, want ErrManyHosts", err)
		}
	})

	t.Run("capabilities still needs --host", func(t *testing.T) {
		if _, err := run(t, handlers, "capabilities"); err == nil {
			t.Fatal("capabilities ran with no --host")
		}
	})

	t.Run("capabilities prints yes and no for each event", func(t *testing.T) {
		out, err := run(t, handlers, "capabilities", "--host", "codex")
		if err != nil {
			t.Fatal(err)
		}
		want := []string{
			string(shareddomain.EventSessionStarted) + " yes",
			string(shareddomain.EventPromptSubmitted) + " yes",
			string(shareddomain.EventToolBefore) + " yes",
			string(shareddomain.EventToolAfter) + " no",
			string(shareddomain.EventTurnStopping) + " yes",
			string(shareddomain.EventSubagentStarted) + " no",
			string(shareddomain.EventSubagentStopped) + " no",
			string(shareddomain.EventToolFailed) + " no",
			string(shareddomain.EventSessionEnded) + " no",
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != len(want) {
			t.Fatalf("lines = %q, want %d", lines, len(want))
		}
		for i, line := range lines {
			if strings.Join(strings.Fields(line), " ") != want[i] {
				t.Fatalf("line %d = %q, want %q", i, line, want[i])
			}
		}
	})

	t.Run("capabilities of a full host has no no", func(t *testing.T) {
		out, err := run(t, handlers, "capabilities", "--host", "claude")
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out, " no") {
			t.Fatalf("output has a no line:\n%s", out)
		}
	})

	t.Run("unknown host is an error", func(t *testing.T) {
		for _, sub := range []string{"install", "capabilities"} {
			if _, err := run(t, handlers, sub, "--host", "vim"); !errors.Is(err, cli.ErrUnknownHost) {
				t.Fatalf("%s error = %v, want ErrUnknownHost", sub, err)
			}
		}
		if len(installed) != 1 {
			t.Fatalf("install ran for an unknown host: %v", installed)
		}
	})
}
