package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/shared/infrastructure/telemetry"
)

const (
	exitOK    = 0
	exitUsage = 2
)

const (
	commandName    = "harness"
	commandVersion = "version"
)

var errNoCommand = errors.New("no command given")

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
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		fmt.Fprint(stderr, root.UsageString())
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
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(*cobra.Command, []string) error {
			return errNoCommand
		},
	}
	root.AddCommand(&cobra.Command{
		Use:   commandVersion,
		Short: "Print the build version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", commandName, version)
		},
	})
	return root
}
