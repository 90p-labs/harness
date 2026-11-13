package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	paneldomain "github.com/90p-labs/harness/internal/panel/domain"
	panelcli "github.com/90p-labs/harness/internal/panel/ui/cli"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
)

// fakeLifecycle records the calls and never spawns a process.
type fakeLifecycle struct {
	starts   []int
	startErr error
	instance paneldomain.Instance
	statErr  error
}

func (f *fakeLifecycle) lifecycle() panelcli.Lifecycle {
	return panelcli.Lifecycle{
		Start: func(_ context.Context, port int) error {
			f.starts = append(f.starts, port)
			return f.startErr
		},
		Stop: func(context.Context) error { return nil },
		Status: func(context.Context) (paneldomain.Instance, error) {
			return f.instance, f.statErr
		},
	}
}

func panelConfig(enabled bool, open string, autostart bool) config.Config {
	return config.Config{Panel: config.Panel{Enabled: enabled, Open: open, Autostart: autostart}}
}

func TestPanelOpener(t *testing.T) {
	const runID = "20261113-201204-r4"
	instance := paneldomain.Instance{PID: 7, Port: 4100, URL: "http://127.0.0.1:4100/"}

	cases := []struct {
		name    string
		cfg     config.Config
		wantSet bool
	}{
		{"disabled with browser", panelConfig(false, "browser", false), false},
		{"disabled with host", panelConfig(false, "host", false), false},
		{"enabled with browser", panelConfig(true, "browser", false), false},
		{"enabled with host", panelConfig(true, "host", false), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeLifecycle{instance: instance}
			var stderr bytes.Buffer

			open := panelOpener(tc.cfg, fake.lifecycle(), &stderr)

			if (open != nil) != tc.wantSet {
				t.Fatalf("opener set = %v, want %v", open != nil, tc.wantSet)
			}
			if open == nil {
				return
			}
			if err := open(t.Context(), runID); err != nil {
				t.Fatal(err)
			}
			if len(fake.starts) != 1 || fake.starts[0] >= 0 {
				t.Errorf("starts = %v, want one start with the configured port", fake.starts)
			}
			if want := "panel: http://127.0.0.1:4100/runs/" + runID + "\n"; stderr.String() != want {
				t.Errorf("stderr = %q, want %q", stderr.String(), want)
			}
		})
	}

	t.Run("a start failure warns and returns the error", func(t *testing.T) {
		boom := errors.New("port busy")
		fake := &fakeLifecycle{startErr: boom}
		var stderr bytes.Buffer

		err := panelOpener(panelConfig(true, "host", false), fake.lifecycle(), &stderr)(t.Context(), runID)

		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want %v", err, boom)
		}
		for _, want := range []string{"panel: start failed: port busy", "panel: opening the browser"} {
			if !strings.Contains(stderr.String(), want) {
				t.Errorf("stderr = %q, want it to contain %q", stderr.String(), want)
			}
		}
		if strings.Contains(stderr.String(), "runs/") {
			t.Errorf("stderr = %q, want no run URL", stderr.String())
		}
	})

	t.Run("a status failure warns and returns the error", func(t *testing.T) {
		fake := &fakeLifecycle{statErr: paneldomain.ErrStopped}
		var stderr bytes.Buffer

		err := panelOpener(panelConfig(true, "host", false), fake.lifecycle(), &stderr)(t.Context(), runID)

		if !errors.Is(err, paneldomain.ErrStopped) || !strings.Contains(stderr.String(), "panel: opening the browser") {
			t.Errorf("err = %v, stderr = %q, want the error and the browser notice", err, stderr.String())
		}
	})
}

func TestPanelAutostart(t *testing.T) {
	instance := paneldomain.Instance{URL: "http://127.0.0.1:4100/"}

	for name, cfg := range map[string]config.Config{
		"disabled":      panelConfig(false, "browser", true),
		"no autostart":  panelConfig(true, "browser", false),
		"both disabled": panelConfig(false, "browser", false),
	} {
		t.Run(name+" sets no handler", func(t *testing.T) {
			if panelAutostart(cfg, (&fakeLifecycle{}).lifecycle()) != nil {
				t.Error("handler set, want nil")
			}
		})
	}

	t.Run("enabled with autostart starts and returns the line", func(t *testing.T) {
		fake := &fakeLifecycle{instance: instance}

		line, err := panelAutostart(panelConfig(true, "browser", true), fake.lifecycle())(t.Context())

		if err != nil || line != "panel: http://127.0.0.1:4100/" || len(fake.starts) != 1 || fake.starts[0] >= 0 {
			t.Errorf("line = %q, err = %v, starts = %v", line, err, fake.starts)
		}
	})

	t.Run("a start failure returns the error", func(t *testing.T) {
		boom := errors.New("port busy")
		fake := &fakeLifecycle{startErr: boom}

		line, err := panelAutostart(panelConfig(true, "browser", true), fake.lifecycle())(t.Context())

		if !errors.Is(err, boom) || line != "" {
			t.Errorf("line = %q, err = %v, want the error", line, err)
		}
	})
}
