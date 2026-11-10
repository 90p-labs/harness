package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/run/application/query/report"
	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	commandReport = "report"

	runLineFormatShort   = "run %s phase=%s"
	reportReworkFormat   = "rework rounds: %d"
	reportAmendsFormat   = "amends: %d"
	reportBlocksFormat   = "stop blocks: %d"
	reportClassesPrefix  = "rework classes: "
	reportMinutesPrefix  = "minutes: "
	reportTracePrefix    = "trace: "
	noReworkClasses      = "none"
	repeatClassesPrefix  = "repeat rework classes: "
	repeatClassesWarning = "warning: the repeat rework classes are not available: %v\n"
	classCountFormat     = "%s %d"
	minutesFormat        = "%s %.1f"
	listSeparator        = ", "
)

func reportCommand(connect Connect) *cobra.Command {
	return &cobra.Command{
		Use:   commandReport + " [<run id>]",
		Short: "Print the rework, amend, stop block and phase minute counts of a run",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				query := report.Query{Key: h.Key}
				if len(args) == 1 {
					query.RunID = args[0]
				}
				got, err := h.Report.Handle(ctx, query)
				if err != nil {
					return err
				}
				return printLines(cmd.OutOrStdout(), reportLines(got))
			})
		},
	}
}

func reportLines(r report.Report) []string {
	classes := noReworkClasses
	if len(r.Classes) > 0 {
		classes = classCounts(r.Classes)
	}
	minutes := make([]string, len(r.Minutes))
	for i, m := range r.Minutes {
		minutes[i] = fmt.Sprintf(minutesFormat, m.Phase, m.Minutes)
	}
	return []string{
		fmt.Sprintf(runLineFormatShort, r.ID, r.Phase),
		fmt.Sprintf(reportReworkFormat, r.ReworkRounds),
		fmt.Sprintf(reportAmendsFormat, r.Amends),
		fmt.Sprintf(reportBlocksFormat, r.StopBlocks),
		reportClassesPrefix + classes,
		reportMinutesPrefix + strings.Join(minutes, listSeparator),
		reportTracePrefix + r.TraceID,
	}
}

func classCounts(counts []domain.ClassCount) string {
	parts := make([]string, len(counts))
	for i, c := range counts {
		parts[i] = fmt.Sprintf(classCountFormat, c.Class, c.Count)
	}
	return strings.Join(parts, listSeparator)
}

func printRepeatClasses(ctx context.Context, out, warn io.Writer, h Handlers) error {
	counts, err := h.Report.RepeatClasses(ctx, h.Key.Repo)
	if err != nil {
		_, err = fmt.Fprintf(warn, repeatClassesWarning, err)
		return err
	}
	if len(counts) == 0 {
		return nil
	}
	return printLines(out, []string{repeatClassesPrefix + classCounts(counts)})
}

func printLines(out io.Writer, lines []string) error {
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	return nil
}
