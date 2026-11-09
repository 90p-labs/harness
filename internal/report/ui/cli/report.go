package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/report/application/command/fileissue"
	"github.com/90p-labs/harness/internal/report/domain"
)

const (
	commandReport  = "report"
	flagTitle      = "title"
	flagBodyFile   = "body-file"
	flagDryRun     = "dry-run"
	missingFlagMsg = "--%s is required"
)

type File func(ctx context.Context, draft domain.Draft, dryRun bool) (fileissue.Result, error)

func NewCommand(file File) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandReport,
		Short: "File a harness bug in jorge07/harness; with --dry-run, print the checked and redacted issue",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			for _, name := range []string{flagTitle, flagBodyFile} {
				if value, _ := cmd.Flags().GetString(name); value == "" {
					return fmt.Errorf(missingFlagMsg, name)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			title, _ := cmd.Flags().GetString(flagTitle)
			path, _ := cmd.Flags().GetString(flagBodyFile)
			dryRun, _ := cmd.Flags().GetBool(flagDryRun)
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			result, err := file(cmd.Context(), domain.Draft{Title: title, Body: string(body)}, dryRun)
			if err != nil {
				return err
			}
			if dryRun {
				_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s\n\n%s", result.Issue.Title, result.Issue.Body)
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), result.URL)
			return err
		},
	}
	cmd.Flags().String(flagTitle, "", "the issue title")
	cmd.Flags().String(flagBodyFile, "", "the markdown draft with every section of the issue template")
	cmd.Flags().Bool(flagDryRun, false, "print the issue and file nothing")
	return cmd
}
