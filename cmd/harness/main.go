package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	guardcli "github.com/90p-labs/harness/internal/guard/ui/cli"
	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/telemetry"
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

type refusalError struct {
	error
	hint string
}

func (e refusalError) Error() string { return e.error.Error() + "\n" + e.hint }

func (e refusalError) Unwrap() error { return e.error }

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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	shutdown, err := telemetry.Setup(ctx, version)
	reportTelemetry(stderr, err)
	defer func() { reportTelemetry(stderr, shutdown(ctx)) }()

	root := newRootCommand()
	// cobra reads os.Args[1:] when the args slice is nil.
	root.SetArgs(append([]string{}, args...))
	root.SetIn(stdin)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if failed, err := root.ExecuteContextC(telemetry.ParentFromEnv(ctx)); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		switch {
		case errors.As(err, new(refusalError)):
			return exitUsage
		case errors.As(err, new(usageError)):
			fmt.Fprint(stderr, failed.UsageString())
			return exitUsage
		}
		return exitRuntime
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
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			resolveRun(cmd)
			return nil
		},
		RunE: func(*cobra.Command, []string) error {
			return usageError{errNoCommand}
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return usageError{err} })
	root.PersistentFlags().String(config.FlagConfig, "", "path of the config file")
	root.PersistentFlags().String(config.FlagStateDSN, "", "state database DSN")
	root.AddCommand(newConfigCommand(), newRunCommand(root.PersistentFlags()), newPlanCommand(root.PersistentFlags()), newDiagramCommand(), newHostCommand(root.PersistentFlags()))
	root.AddCommand(newVerifyCommand(root.PersistentFlags()), newGatesCommand(root.PersistentFlags()), newLintCommand(root.PersistentFlags(), version), newLiveCommand(root.PersistentFlags()))
	root.AddCommand(newInitCommand(root.PersistentFlags(), version))
	root.AddCommand(newReportCommand(root.PersistentFlags()))
	root.AddCommand(guardcli.NewPRCommand())
	root.AddCommand(&cobra.Command{
		Use:   commandVersion,
		Short: "Print the build version",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", commandName, version)
		},
	})
	wrapArgs(root)
	// Claude Code reads exit status 2 of a hook as a block, so no hook error is a usage error.
	root.AddCommand(newHookCommand(root.PersistentFlags()))
	return root
}
