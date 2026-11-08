package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/shared/infrastructure/config"
	"github.com/jorge07/harness/internal/shared/infrastructure/telemetry"
)

const (
	exitOK      = 0
	exitRuntime = 1
	exitUsage   = 2
)

const (
	commandName    = "harness"
	commandVersion = "version"
	commandConfig  = "config"
)

var errNoCommand = errors.New("no command given")

type usageError struct{ error }

func (e usageError) Unwrap() error { return e.error }

func usageArgs(check cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := check(cmd, args); err != nil {
			return usageError{err}
		}
		return nil
	}
}

// version is set at build time with -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	shutdown, err := telemetry.Setup(ctx, version)
	reportTelemetry(stderr, err)
	defer func() { reportTelemetry(stderr, shutdown(ctx)) }()

	root := newRootCommand()
	// cobra reads os.Args[1:] when the args slice is nil.
	root.SetArgs(append([]string{}, args...))
	root.SetOut(stdout)
	root.SetErr(stderr)
	if failed, err := root.ExecuteContextC(ctx); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		if !errors.As(err, new(usageError)) {
			return exitRuntime
		}
		fmt.Fprint(stderr, failed.UsageString())
		return exitUsage
	}
	return exitOK
}

func reportTelemetry(stderr io.Writer, err error) {
	if err != nil {
		fmt.Fprintf(stderr, "telemetry: %s\n", strings.ReplaceAll(err.Error(), "\n", "; "))
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           commandName,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			return usageError{errNoCommand}
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })
	root.PersistentFlags().String(config.FlagConfig, "", "path of the config file")
	root.PersistentFlags().String(config.FlagStateDSN, "", "state database DSN")
	root.AddCommand(newConfigCommand(), newRunCommand(root.PersistentFlags()), newPlanCommand(root.PersistentFlags()))
	root.AddCommand(&cobra.Command{
		Use:   commandVersion,
		Short: "Print the build version",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", commandName, version)
		},
	})
	wrapArgs(root)
	return root
}
