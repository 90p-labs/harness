package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/live/application/command/check"
)

const (
	commandLive  = "live"
	commandCheck = "check"

	resultFormat      = "%s %s %s"
	resultErrorFormat = "%s %s %s: %v"
)

type Check func(ctx context.Context) ([]check.Result, error)

func NewCommand(run Check) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandLive,
		Short: "Follow the runs released to production",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   commandCheck,
		Short: "Check each live run once",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			results, err := run(cmd.Context())
			for _, line := range Lines(results) {
				if _, werr := fmt.Fprintln(cmd.OutOrStdout(), line); werr != nil {
					return werr
				}
			}
			return err
		},
	})
	return cmd
}

func Lines(results []check.Result) []string {
	lines := make([]string, len(results))
	for i, r := range results {
		if r.Err != nil {
			lines[i] = fmt.Sprintf(resultErrorFormat, r.RunID, r.Phase, r.Outcome, r.Err)
			continue
		}
		lines[i] = fmt.Sprintf(resultFormat, r.RunID, r.Phase, r.Outcome)
	}
	return lines
}
