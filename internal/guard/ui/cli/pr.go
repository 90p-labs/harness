package cli

import (
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
	missingFlagMsg = "--%s is required"
	bodyOK         = "ok"
)

var errBodyProblems = errors.New("the pull request body has problems")

func NewPRCommand() *cobra.Command {
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
	pr.AddCommand(check)
	return pr
}
