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
	learnNothing         = "learn: no convention candidates and no evidence"
	learnEvidenceFormat  = "evidence: %s: %s"
	learnEvidenceNoText  = "evidence: %s"
	learnNextEvidence    = "next: add the user corrections and the lessons that cite a line above, then ask the Conventions question; with nothing to ask, start pr-open (references/learning.md)"
	learnNextNone        = "next: with no user correction, start pr-open"
)

func learnCommand(connect Connect) *cobra.Command {
	return &cobra.Command{
		Use:   commandLearn + " [<run id>]",
		Short: "List the convention candidates and the run evidence of a run",
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
	if len(l.Candidates) == 0 && len(l.Evidence) == 0 {
		return append(lines, learnNothing, learnNextNone)
	}
	if len(l.Candidates) == 0 {
		lines = append(lines, learnNoCandidates)
	}
	for _, c := range l.Candidates {
		lines = append(lines, candidateLine(c))
	}
	for _, e := range l.Evidence {
		lines = append(lines, evidenceLine(e))
	}
	return append(lines, learnNextEvidence)
}

func evidenceLine(e domain.RunEvidence) string {
	if e.Text == "" {
		return fmt.Sprintf(learnEvidenceNoText, e.ID)
	}
	return fmt.Sprintf(learnEvidenceFormat, e.ID, e.Text)
}

func candidateLine(c domain.Candidate) string {
	if c.Location == "" {
		return fmt.Sprintf(learnNoLocation, c.Source, c.ID, c.Text)
	}
	return fmt.Sprintf(learnCandidateFormat, c.Source, c.ID, c.Location, c.Text)
}
