package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/plan/application/command/renderview"
	"github.com/90p-labs/harness/internal/plan/application/command/submitplan"
	"github.com/90p-labs/harness/internal/plan/application/query/checkplan"
	"github.com/90p-labs/harness/internal/plan/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
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
	cmd := &cobra.Command{Use: commandPlan, Short: "Work with the plan", Long: planHelp()}
	cmd.AddCommand(checkCommand(connect), viewCommand(connect))
	return cmd
}

func planHelp() string {
	signalsHeader := "| " + strings.Join(domain.SignalsHeader, " | ") + " |"
	criteriaHeader := "| " + strings.Join(domain.CriteriaHeader, " | ") + " |"
	sketch := "![title](path" + domain.SketchExt + ")"
	return strings.Join([]string{
		"Write the plan as markdown. `harness plan check` lists each problem with its line and rule.",
		fmt.Sprintf("Level-2 headings: %s.", strings.Join(domain.TopHeadings, ", ")),
		fmt.Sprintf("## %s: `- %s` or one `- question` on each line. The answers move to ## %s as `- <date>: <question>: <answer>`.", domain.HeadingOpenQuestions, domain.OpenQuestionsNone, domain.HeadingDecisions),
		fmt.Sprintf("## %s ends with %d level-3 headings in this order: %s.", domain.HeadingSpecs, len(domain.FinalHeadings), strings.Join(domain.FinalHeadings, ", ")),
		fmt.Sprintf("Each other level-3 heading there needs a %s line with %s. Each criterion is in one of them.", domain.CriteriaKey, domain.FormSpecCriteria),
		fmt.Sprintf("## %s: an optional line `%s %s` or `%s %s`.", domain.HeadingGoal, domain.TrackPrefix, shareddomain.TrackLight, domain.TrackPrefix, shareddomain.TrackFull),
		fmt.Sprintf("## %s: the table header is `%s`, numbered from 1. A proof is one command in backticks, with no placeholder.", domain.HeadingCriteria, criteriaHeader),
		fmt.Sprintf("Task: the heading `### %s`, then one bullet `- Field: value` for each of: %s.", domain.FormTaskHeading, strings.Join(domain.TaskFields, ", ")),
		fmt.Sprintf("%s is one of: %s. %s lists paths in backticks; a folder ends in /.", domain.FieldSide, strings.Join(domain.TaskSides, ", "), domain.FieldOwned),
		fmt.Sprintf("%s starts with %s. %s is %s.", domain.FieldDoD, domain.FormDoD, domain.FieldDependsOn, domain.FormDependsOn),
		fmt.Sprintf("%s is optional and names the model of the worker agent for the task, such as haiku or sonnet.", domain.FieldModel),
		"Each criterion is in a DoD line. Each task is in a commit. Each tree file is in the Owned files of a task.",
		fmt.Sprintf("Trees: a `%s` fence under %s and %s. The first line is the root and ends in /. Indent %d characters for each level.", domain.FenceTree, domain.HeadingCodeTree, domain.HeadingTestTree, domain.TreeLevelWidth),
		fmt.Sprintf("Each file ends with a tree mark: %s. A folder takes no mark.", domain.TreeMarks),
		"The diagram marks are another set: run `harness diagram --help`.",
		fmt.Sprintf("%s: one line for each commit, %s, with a conventional commit subject.", domain.HeadingCommits, domain.FormCommit),
		fmt.Sprintf("%s: one line for each PR, %s. Exactly one PR is (this run).", domain.HeadingPRs, domain.FormPR),
		fmt.Sprintf("## %s is required when the code tree has a file under %s.", domain.HeadingSignals, strings.Join(domain.SignalFolders, ", ")),
		fmt.Sprintf("Its table header is `%s`. Breach is an operator and a number. Window is a Go duration.", signalsHeader),
		fmt.Sprintf("Required diagrams, each in a mermaid fence (rule %s):", domain.RuleDiagramRequired),
		fmt.Sprintf("  %s: a file under %s or ending %s.", domain.DiagramER, domain.FolderMigrations, domain.SuffixSQL),
		fmt.Sprintf("  %s: a contract file (%s), or %d new files under %s.", domain.DiagramClass, domain.ContractNames, domain.NewDomainFilesForClass, domain.FolderDomain),
		fmt.Sprintf("  %s: a new file under %s or %s.", domain.DiagramSequence, domain.FolderHTTP, domain.FolderConsumer),
		fmt.Sprintf("  A frontend task needs a sketch link `%s` to a file in the plan folder.", sketch),
		fmt.Sprintf("The syntax parse covers only %s and %s. The mark check covers every marked kind: it needs one mark.", domain.DiagramState, domain.DiagramClass),
		fmt.Sprintf("A plan with ## %s is a multi-run plan. Headings: %s, %s, optional %s and %s. Run: `### %s`, then a bullet for each of: %s, then %s.", domain.HeadingRuns, domain.HeadingGoal, domain.HeadingRuns, domain.HeadingMeasurement, domain.HeadingDecisions, domain.FormRunHeading, strings.Join(domain.ProgramFields, ", "), domain.FormScope),
		fmt.Sprintf("In a run, %s is %s. Each dependency names a run of an earlier wave. Runs of one wave own no common path. `harness run plan` takes only a single-run plan.", domain.FieldDependsOn, domain.FormRunDepends),
		fmt.Sprintf("A sentence has %d words at most. Use no em dash.", domain.MaxSentenceWords),
	}, "\n")
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
