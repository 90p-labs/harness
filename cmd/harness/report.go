package main

import (
	"context"
	"errors"
	"os"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/jorge07/harness/internal/report"
	"github.com/jorge07/harness/internal/report/application/command/fileissue"
	reportdomain "github.com/jorge07/harness/internal/report/domain"
	reportcli "github.com/jorge07/harness/internal/report/ui/cli"
)

const reportHint = "fill each section of the issue template, then run harness report again"

func newReportCommand(flags *pflag.FlagSet) *cobra.Command {
	return reportcli.NewCommand(func(ctx context.Context, draft reportdomain.Draft, dryRun bool) (fileissue.Result, error) {
		home, _ := os.UserHomeDir()
		result, err := report.NewFileIssue().Handle(ctx, fileissue.Command{
			Draft: draft,
			Environment: reportdomain.Environment{
				Version: version, Platform: runtime.GOOS + "/" + runtime.GOARCH, Run: runLine(ctx, flags), Home: home,
			},
			DryRun: dryRun,
		})
		if errors.Is(err, reportdomain.ErrDraftIncomplete) {
			return result, refusalError{error: err, hint: reportHint}
		}
		return result, err
	})
}

// A report must work in a broken setup, so an environment or store error gives no run instead of a failure.
func runLine(ctx context.Context, flags *pflag.FlagSet) string {
	env, err := newEnvironment(ctx, flags)
	if err != nil {
		return reportdomain.RunNone
	}
	ref, err := activeRunRef(env)(ctx)
	if err != nil {
		return reportdomain.RunNone
	}
	return reportdomain.RunLine(ref.ID, ref.Phase)
}
