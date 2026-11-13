package main

import (
	"context"
	"fmt"
	"io"

	panelcli "github.com/90p-labs/harness/internal/panel/ui/cli"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
)

// configuredPort makes the lifecycle use panel.port.
const configuredPort = -1

// panelOpener returns the host open of the plan view. It is nil unless the
// panel is enabled with panel.open: host, so the plan opens in the browser.
// The func starts the panel and prints the run URL on stderr. On a failure it
// prints a warning and returns the error, so the plan module opens the browser.
func panelOpener(cfg config.Config, life panelcli.Lifecycle, stderr io.Writer) func(ctx context.Context, run string) error {
	if !cfg.Panel.Enabled || cfg.Panel.Open != config.PanelOpenHost {
		return nil
	}
	return func(ctx context.Context, run string) error {
		url, err := startedURL(ctx, life)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "panel: start failed: %v\npanel: opening the browser\n", err)
			return err
		}
		_, err = fmt.Fprintf(stderr, "panel: %sruns/%s\n", url, run)
		return err
	}
}

// panelAutostart returns the AfterStart handler of the run start. It is nil
// unless the panel is enabled with panel.autostart.
func panelAutostart(cfg config.Config, life panelcli.Lifecycle) func(ctx context.Context) (string, error) {
	if !cfg.Panel.Enabled || !cfg.Panel.Autostart {
		return nil
	}
	return func(ctx context.Context) (string, error) {
		url, err := startedURL(ctx, life)
		if err != nil {
			return "", err
		}
		return "panel: " + url, nil
	}
}

// startedURL starts the panel, which is idempotent, and reads its URL.
func startedURL(ctx context.Context, life panelcli.Lifecycle) (string, error) {
	if err := life.Start(ctx, configuredPort); err != nil {
		return "", err
	}
	instance, err := life.Status(ctx)
	if err != nil {
		return "", err
	}
	return instance.URL, nil
}
