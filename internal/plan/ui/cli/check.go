package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/plan/application/command/renderview"
	"github.com/jorge07/harness/internal/plan/application/command/submitplan"
	"github.com/jorge07/harness/internal/plan/application/query/checkplan"
	"github.com/jorge07/harness/internal/plan/domain"
)

const (
	commandPlan  = "plan"
	commandCheck = "check"
	flagFile     = "file"
)

var ErrNoPlanFile = errors.New("no plan file: give --file or start a run")

type Handlers struct {
	Check       checkplan.Handler
	Submit      submitplan.Handler
	View        renderview.Handler
	DefaultPlan func(ctx context.Context) (checkplan.Query, error)
	DefaultView func(ctx context.Context) (renderview.Command, error)
}

type Connect func(ctx context.Context) (Handlers, func() error, error)

func NewCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{Use: commandPlan, Short: "Work with the plan"}
	cmd.AddCommand(checkCommand(connect), viewCommand(connect))
	return cmd
}

func checkCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandCheck,
		Short: "Check the plan",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) (err error) {
			handlers, closeStore, err := connect(cmd.Context())
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, closeStore()) }()
			query, err := queryFor(cmd, handlers)
			if err != nil {
				return err
			}
			report, err := handlers.Check.Handle(cmd.Context(), query)
			if err != nil {
				return err
			}
			for _, p := range report.Problems {
				fmt.Fprintln(cmd.OutOrStdout(), p.String())
			}
			for _, note := range report.Notes {
				fmt.Fprintln(cmd.OutOrStdout(), note)
			}
			if len(report.Problems) > 0 {
				return domain.ErrPlanHasProblems
			}
			return nil
		},
	}
	cmd.Flags().String(flagFile, "", "the plan file, default the plan of the active run")
	return cmd
}

func queryFor(cmd *cobra.Command, handlers Handlers) (checkplan.Query, error) {
	if path, _ := cmd.Flags().GetString(flagFile); path != "" {
		return checkplan.Query{Path: path}, nil
	}
	return handlers.DefaultPlan(cmd.Context())
}
