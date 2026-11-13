package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/run/application/query/learn"
	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	commandLearn = "learn"

	learnRunFormat       = "run: %s"
	learnCandidateFormat = "candidate: %s %s %s: %s"
	learnNoLocation      = "candidate: %s %s: %s"
	learnNoCandidates    = "learn: no convention candidates"
	learnNextCandidates  = "next: add the user corrections, then ask the Conventions question (references/learning.md)"
	learnNextNone        = "next: with no user correction, start pr-open"
)

func learnCommand(connect Connect) *cobra.Command {
	return &cobra.Command{
		Use:   commandLearn + " [<run id>]",
		Short: "List the convention candidates of a run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				query := learn.Query{Key: h.Key}
				if len(args) == 1 {
					query.RunID = args[0]
				}
				got, err := h.Learn.Handle(ctx, query)
				if err != nil {
					return err
				}
				return printLines(cmd.OutOrStdout(), learnLines(got))
			})
		},
	}
}

func learnLines(l learn.Learn) []string {
	lines := []string{fmt.Sprintf(learnRunFormat, l.ID)}
	if len(l.Candidates) == 0 {
		return append(lines, learnNoCandidates, learnNextNone)
	}
	for _, c := range l.Candidates {
		lines = append(lines, candidateLine(c))
	}
	return append(lines, learnNextCandidates)
}

func candidateLine(c domain.Candidate) string {
	if c.Location == "" {
		return fmt.Sprintf(learnNoLocation, c.Source, c.ID, c.Text)
	}
	return fmt.Sprintf(learnCandidateFormat, c.Source, c.ID, c.Location, c.Text)
}
