package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	commandLint     = "lint"
	commandInit     = "init"
	commandBaseline = "baseline"
	flagJSON        = "json"

	findingFormat = "%s %s %s:%d %s\n"
	noPacksLine   = "no lint pack is set\n"
)

var ErrFindings = errors.New("lint found error findings")

type Handlers struct {
	Lint     func(ctx context.Context) (shareddomain.Report, error)
	Init     func(ctx context.Context) error
	Baseline func(ctx context.Context) error
	NoPacks  bool
}

type Connect func(ctx context.Context) (Handlers, error)

func NewLintCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandLint,
		Short: "Check the code against the lint packs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			asJSON, _ := cmd.Flags().GetBool(flagJSON)
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				report, err := h.Lint(ctx)
				if err != nil {
					return err
				}
				if asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
				}
				if h.NoPacks {
					_, err := fmt.Fprint(cmd.OutOrStdout(), noPacksLine)
					return err
				}
				return printFindings(cmd.OutOrStdout(), report)
			})
		},
	}
	cmd.Flags().Bool(flagJSON, false, "print the report as JSON")
	cmd.AddCommand(
		subcommand(commandInit, "Write the engine files of the active packs", connect, func(h Handlers) func(context.Context) error { return h.Init }),
		subcommand(commandBaseline, "Keep the lint baseline ratchet", connect, func(h Handlers) func(context.Context) error { return h.Baseline }),
	)
	return cmd
}

func subcommand(use, short string, connect Connect, pick func(Handlers) func(context.Context) error) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				return pick(h)(ctx)
			})
		},
	}
}

func printFindings(out io.Writer, report shareddomain.Report) error {
	failed := false
	for _, f := range report.Findings {
		if _, err := fmt.Fprintf(out, findingFormat, f.Severity, f.Rule, f.File, f.Line, f.Message); err != nil {
			return err
		}
		failed = failed || f.Severity == shareddomain.SeverityError
	}
	if failed {
		return ErrFindings
	}
	return nil
}

func withHandlers(cmd *cobra.Command, connect Connect, run func(context.Context, Handlers) error) error {
	handlers, err := connect(cmd.Context())
	if err != nil {
		return err
	}
	return run(cmd.Context(), handlers)
}
