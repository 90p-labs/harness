package browser

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

const (
	envNoBrowser = "HARNESS_NO_BROWSER"
	osDarwin     = "darwin"
	openDarwin   = "open"
	openOther    = "xdg-open"
)

type Opener struct {
	GOOS   string
	Getenv func(string) string
	Run    func(ctx context.Context, name string, args ...string) error
}

func New() Opener {
	return Opener{GOOS: runtime.GOOS, Getenv: os.Getenv, Run: run}
}

func run(ctx context.Context, name string, args ...string) error {
	return exec.CommandContext(ctx, name, args...).Run()
}

func (o Opener) Open(ctx context.Context, page string) error {
	if o.Getenv(envNoBrowser) != "" {
		return nil
	}
	name := openOther
	if o.GOOS == osDarwin {
		name = openDarwin
	}
	if err := o.Run(ctx, name, page); err != nil {
		return fmt.Errorf("open %s with %s: %w", page, name, err)
	}
	return nil
}
