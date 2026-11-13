package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/panel/domain"
)

const testURL = "http://127.0.0.1:4711/"

type fakeLife struct {
	running bool
	version string
	ports   []int
	stops   int
	startErr,
	stopErr error
}

func (f *fakeLife) lifecycle() Lifecycle {
	return Lifecycle{
		Start: func(_ context.Context, port int) error {
			f.ports = append(f.ports, port)
			if f.startErr == nil {
				f.running = true
			}
			return f.startErr
		},
		Stop: func(context.Context) error {
			f.stops++
			f.running = false
			return f.stopErr
		},
		Status: func(context.Context) (domain.Instance, error) {
			if !f.running {
				return domain.Instance{}, domain.ErrStopped
			}
			return domain.Instance{PID: 42, Port: 4711, URL: testURL, Version: f.version}, nil
		},
	}
}

func runCommand(t *testing.T, life Lifecycle, args ...string) (stdout string, err error) {
	t.Helper()
	cmd := NewCommand(func(context.Context, int, func(string)) error { return nil }, life)
	var out bytes.Buffer
	cmd.SilenceErrors, cmd.SilenceUsage = true, true // the root command does the same
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err = cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestLifecycleCommands(t *testing.T) {
	t.Run("start prints the url and passes -1 without the flag", func(t *testing.T) {
		f := &fakeLife{}
		out, err := runCommand(t, f.lifecycle(), "start")
		if err != nil || out != "panel: "+testURL+"\n" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
		if len(f.ports) != 1 || f.ports[0] != -1 {
			t.Fatalf("ports = %v, want [-1]", f.ports)
		}
	})

	t.Run("start passes the flag value, even 0", func(t *testing.T) {
		for _, port := range []string{"8123", "0"} {
			f := &fakeLife{}
			if _, err := runCommand(t, f.lifecycle(), "start", "--port", port); err != nil {
				t.Fatal(err)
			}
			if want := map[string]int{"8123": 8123, "0": 0}[port]; f.ports[0] != want {
				t.Fatalf("--port %s: ports = %v, want [%d]", port, f.ports, want)
			}
		}
	})

	t.Run("start help keeps the default 0", func(t *testing.T) {
		cmd := NewCommand(nil, (&fakeLife{}).lifecycle())
		start, _, err := cmd.Find([]string{"start"})
		if err != nil {
			t.Fatal(err)
		}
		if f := start.Flags().Lookup("port"); f == nil || f.DefValue != "0" {
			t.Fatalf("port flag = %+v, want default 0", f)
		}
	})

	t.Run("start finds another version running", func(t *testing.T) {
		f := &fakeLife{running: true, version: "1.0.0"}
		life := f.lifecycle()
		life.Version = "2.0.0"
		out, err := runCommand(t, life, "start")
		want := "panel: running version 1.0.0, run harness panel stop to switch to 2.0.0\npanel: " + testURL + "\n"
		if err != nil || out != want || len(f.ports) != 0 {
			t.Fatalf("out = %q, err = %v, ports = %v", out, err, f.ports)
		}
	})

	t.Run("start prepares before the version check and status does not", func(t *testing.T) {
		f := &fakeLife{running: true, version: "1.0.0"}
		life := f.lifecycle()
		life.Version = "2.0.0"
		prepared := 0
		life.Prepare = func(context.Context) error { prepared++; return nil }
		out, err := runCommand(t, life, "start")
		if err != nil || prepared != 1 || len(f.ports) != 0 || !strings.Contains(out, "running version 1.0.0") {
			t.Fatalf("out = %q, err = %v, prepared = %d, ports = %v", out, err, prepared, f.ports)
		}
		if _, err := runCommand(t, life, "status"); err != nil {
			t.Fatal(err)
		}
		if _, err := runCommand(t, life, "stop"); err != nil {
			t.Fatal(err)
		}
		if prepared != 1 {
			t.Errorf("status or stop prepared: %d", prepared)
		}
	})

	t.Run("start stops on a prepare error", func(t *testing.T) {
		f := &fakeLife{}
		life := f.lifecycle()
		life.Prepare = func(context.Context) error { return errors.New("prep") }
		if _, err := runCommand(t, life, "start"); err == nil || len(f.ports) != 0 {
			t.Fatalf("err = %v, ports = %v", err, f.ports)
		}
	})

	t.Run("start finds the same version running", func(t *testing.T) {
		f := &fakeLife{running: true, version: "2.0.0"}
		life := f.lifecycle()
		life.Version = "2.0.0"
		out, err := runCommand(t, life, "start")
		if err != nil || out != "panel: "+testURL+"\n" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})

	t.Run("start failure prints no url", func(t *testing.T) {
		f := &fakeLife{startErr: errors.New("boom")}
		out, err := runCommand(t, f.lifecycle(), "start")
		if err == nil || !strings.Contains(err.Error(), "boom") || strings.Contains(out, "panel: ") {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})

	t.Run("status of a running panel", func(t *testing.T) {
		out, err := runCommand(t, (&fakeLife{running: true}).lifecycle(), "status")
		if err != nil || out != "panel: "+testURL+" pid 42\n" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})

	t.Run("status of a stopped panel fails", func(t *testing.T) {
		out, err := runCommand(t, (&fakeLife{}).lifecycle(), "status")
		if !errors.Is(err, domain.ErrStopped) || out != "panel: stopped\n" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})

	t.Run("status takes no port", func(t *testing.T) {
		if _, err := runCommand(t, (&fakeLife{}).lifecycle(), "status", "--port", "1"); err == nil {
			t.Fatal("status accepted --port")
		}
	})

	t.Run("stop ends the panel", func(t *testing.T) {
		f := &fakeLife{running: true}
		out, err := runCommand(t, f.lifecycle(), "stop")
		if err != nil || f.stops != 1 || out != "panel: stopped\n" {
			t.Fatalf("out = %q, err = %v, stops = %d", out, err, f.stops)
		}
	})

	t.Run("stop failure prints no line", func(t *testing.T) {
		f := &fakeLife{running: true, stopErr: errors.New("stuck")}
		out, err := runCommand(t, f.lifecycle(), "stop")
		if err == nil || out != "" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})
}
