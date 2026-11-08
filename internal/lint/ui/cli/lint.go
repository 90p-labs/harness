package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	commandLint      = "lint"
	commandInit      = "init"
	commandBaseline  = "baseline"
	commandCatalogue = "catalogue"
	flagJSON         = "json"

	findingFormat       = "%s %s %s:%d %s\n"
	catalogueRuleFormat = "%s %s\n"
	noPacksLine         = "no lint pack is set: set lint.packs in .harness.yaml\n"
)

var ErrFindings = errors.New("lint found error findings")

type Handlers struct {
	Lint     func(ctx context.Context) (shareddomain.Report, error)
	Init     func(ctx context.Context) error
	Baseline func(ctx context.Context) error
	NoPacks  bool
}

type Connect func(ctx context.Context) (Handlers, error)

func NewLintCommand(connect Connect, catalogue func() (domain.RuleCatalogue, error)) *cobra.Command {
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
		&cobra.Command{
			Use:   commandInit,
			Short: "Write the engine files of the active packs",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
					if err := h.Init(ctx); err != nil || !h.NoPacks {
						return err
					}
					_, err := fmt.Fprint(cmd.OutOrStdout(), noPacksLine)
					return err
				})
			},
		},
		&cobra.Command{
			Use:   commandBaseline,
			Short: "Keep the lint baseline ratchet",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
					return h.Baseline(ctx)
				})
			},
		},
		newCatalogueCommand(catalogue),
	)
	return cmd
}

func newCatalogueCommand(load func() (domain.RuleCatalogue, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandCatalogue,
		Short: "Print the architecture rule catalogue",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalogue, err := load()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if asJSON, _ := cmd.Flags().GetBool(flagJSON); asJSON {
				return json.NewEncoder(out).Encode(catalogue)
			}
			for _, pack := range catalogue.Packs {
				for _, rule := range pack.Rules {
					if _, err := fmt.Fprintf(out, catalogueRuleFormat, rule.ID, rule.Description); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().Bool(flagJSON, false, "print the catalogue as JSON")
	return cmd
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
