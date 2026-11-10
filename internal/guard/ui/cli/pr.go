package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/guard/domain"
)

const (
	commandPR      = "pr"
	commandCheck   = "check"
	flagBodyFile   = "body-file"
	flagBase       = "base"
	missingFlagMsg = "--%s is required"
	bodyOK         = "ok"

	commandBody           = "body"
	bottomLinePlaceholder = "<bottom line: what the PR changes and what the reviewer must check>"
	whyPlaceholder        = "<why: the problem and the reason for the change>"
	noDiagramLine         = "The change adds no diagram."
	fenceMermaidOpen      = "```mermaid"
	fenceClose            = "```"
)

// BodySource reads what the pull request body needs: the plan of the active run and the branch commit subjects.
type BodySource interface {
	// Plan returns the plan text, or an empty string when there is no active run.
	Plan(ctx context.Context) (string, error)
	// Commits returns the subjects of the commits over the base, oldest first. An empty base means the default branch.
	Commits(ctx context.Context, base string) ([]string, error)
}

var errBodyProblems = errors.New("the pull request body has problems")

func NewPRCommand(source BodySource) *cobra.Command {
	pr := &cobra.Command{Use: commandPR, Short: "Check a pull request before it opens"}
	check := &cobra.Command{
		Use:   commandCheck,
		Short: "Print every problem of a pull request body file; exit 1 when there is one",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			if path, _ := cmd.Flags().GetString(flagBodyFile); path == "" {
				return fmt.Errorf(missingFlagMsg, flagBodyFile)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString(flagBodyFile)
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			problems := domain.BodyProblems(string(body))
			if len(problems) == 0 {
				_, err = fmt.Fprintln(cmd.OutOrStdout(), bodyOK)
				return err
			}
			if _, err := fmt.Fprintln(cmd.OutOrStdout(), strings.Join(problems, "\n")); err != nil {
				return err
			}
			return errBodyProblems
		},
	}
	check.Flags().String(flagBodyFile, "", "the pull request body file")
	body := &cobra.Command{
		Use:   commandBody,
		Short: "Print a pull request body skeleton: placeholders for the bottom line and Why, What from the commits, How from the plan diagrams",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := source.Plan(cmd.Context())
			if err != nil {
				return err
			}
			base, _ := cmd.Flags().GetString(flagBase)
			commits, err := source.Commits(cmd.Context(), base)
			if err != nil {
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), skeleton(plan, commits))
			return err
		},
	}
	body.Flags().String(flagBase, "", "the base branch of the pull request; default: the default branch")
	pr.AddCommand(check, body)
	return pr
}

func skeleton(plan string, commits []string) string {
	var b strings.Builder
	b.WriteString(bottomLinePlaceholder + "\n\n## Why\n\n" + whyPlaceholder + "\n\n## What\n\n")
	for _, subject := range commits {
		b.WriteString("- " + subject + "\n")
	}
	b.WriteString("\n## How\n\n")
	fences := mermaidFences(plan)
	if len(fences) == 0 {
		b.WriteString(noDiagramLine + "\n")
	}
	for i, fence := range fences {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fence)
	}
	return b.String()
}

// mermaidFences returns each mermaid fence of the text, with its fence lines and a closing newline.
func mermaidFences(text string) []string {
	var fences []string
	var current []string
	open := false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case open && trimmed == fenceClose:
			fences = append(fences, strings.Join(append(current, fenceClose), "\n")+"\n")
			current, open = nil, false
		case open:
			current = append(current, line)
		case trimmed == fenceMermaidOpen:
			current, open = []string{fenceMermaidOpen}, true
		}
	}
	return fences
}
