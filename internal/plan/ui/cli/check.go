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

	advisorOn     = "advisor: on, model %s"
	advisorOnBare = "advisor: on"
	advisorPath   = " (light track, risky path)"
	advisorOff    = "advisor: off (light track)"

	waveLineForm  = "wave %d: %s"
	waveTaskForm  = "task %d"
	waveModelForm = "task %d (%s)"
	waveTaskSep   = ", "
)

var ErrNoPlanFile = errors.New("no plan file: give --file or start a run")

type Handlers struct {
	Check  checkplan.Handler
	Submit submitplan.Handler
	View   renderview.Handler
	// AdvisorModels lists the models of the advisor role for the host: the first serves a full track, the second a path trigger.
	AdvisorModels []string
	// RepoTop is the repository top that plan view prints the page path against; empty prints the path as given.
	RepoTop     string
	DefaultPlan func(ctx context.Context) (checkplan.Query, error)
	DefaultView func(ctx context.Context) (renderview.Command, error)
	Open        func(ctx context.Context, page string) error
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
		fmt.Sprintf("## %s: `- %s` or one `- risk` on each line, as %s (rule %s). A test name must be in a proof or a Test first line.", domain.HeadingReviewRisks, domain.RisksNone, domain.FormRiskLine, domain.RuleRiskUnanchored),
		fmt.Sprintf("## %s is required when a tree path has a %s folder. It needs the bullets %s, each with text (rule %s). A single-run plan has %d tasks at most: split a larger change into a multi-run plan with ## %s (rule %s).", domain.HeadingThreatModel, strings.Join(domain.ThreatFolders, " or "), strings.Join(domain.ThreatLabels, ", "), domain.RuleThreatModel, domain.MaxRunTasks, domain.HeadingRuns, domain.RulePlanSize),
		fmt.Sprintf("## %s ends with %d level-3 headings in this order: %s.", domain.HeadingSpecs, len(domain.FinalHeadings), strings.Join(domain.FinalHeadings, ", ")),
		fmt.Sprintf("Each other level-3 heading there needs a %s line with %s. Each criterion is in one of them.", domain.CriteriaKey, domain.FormSpecCriteria),
		fmt.Sprintf("## %s: an optional line `%s %s` or `%s %s`.", domain.HeadingGoal, domain.TrackPrefix, shareddomain.TrackLight, domain.TrackPrefix, shareddomain.TrackFull),
		fmt.Sprintf("## %s: a bug plan has the line `%s %s`. A `fix` commit needs it too (rule %s).", domain.HeadingGoal, domain.CausePrefix, domain.CauseBug, domain.RuleReproduction),
		fmt.Sprintf("## %s goes with that line. Bullets: `- %s: n` and `- %s: `text``, or `- %s: reason`, then `- %s: `TestName`` or `- %s: none: reason`. Then %d to %d lines `%s`, with the result %s, %s or %s. `harness run plan` in planning needs a failed proof that holds the symptom (rule %s).", domain.HeadingReproduction, domain.ReproCriterion, domain.ReproSymptom, domain.ReproHumanOnly, domain.ReproRegression, domain.ReproRegression, domain.MinHypotheses, domain.MaxHypotheses, domain.FormHypothesis, domain.ResultConfirmed, domain.ResultRefuted, domain.ResultUntested, domain.RuleReproductionRed),
		fmt.Sprintf("## %s: the table header is `%s`, numbered from 1. A proof is one command in backticks, with no placeholder.", domain.HeadingCriteria, criteriaHeader),
		fmt.Sprintf("Task: the heading `### %s`, then one bullet `- Field: value` for each of: %s.", domain.FormTaskHeading, strings.Join(domain.TaskFields, ", ")),
		fmt.Sprintf("%s is one of: %s. %s lists paths in backticks; a folder ends in /.", domain.FieldSide, strings.Join(domain.TaskSides, ", "), domain.FieldOwned),
		fmt.Sprintf("%s starts with %s. %s is %s.", domain.FieldDoD, domain.FormDoD, domain.FieldDependsOn, domain.FormDependsOn),
		fmt.Sprintf("%s is optional and names the model of the worker agent for the task, such as haiku or sonnet.", domain.FieldModel),
		fmt.Sprintf("Each criterion is in a DoD line. Each task is in a commit. Each tree file is in the Owned files of a task. A task that names a test needs a seam, as %s (rule %s).", domain.FormSeam, domain.RuleSeamMissing),
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
		fmt.Sprintf("In a run, %s is %s, %s is %s and %s is %s. Each dependency names a run of an earlier wave (rule %s). Runs of one wave own no common path (rule %s). A missing or malformed field is rule %s. `harness run plan` takes only a single-run plan.", domain.FieldWave, domain.FormWave, domain.FieldBranch, domain.FormBranch, domain.FieldDependsOn, domain.FormRunDepends, domain.RuleRunDepends, domain.RuleRunOverlap, domain.RuleRunField),
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
			if report.Program == nil {
				fmt.Fprintln(cmd.OutOrStdout(), advisorLine(report, handlers.AdvisorModels))
				for _, line := range waveLines(report.Doc.Tasks) {
					fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			}
			return nil
		},
	}
	cmd.Flags().String(flagFile, "", "the plan file, default the plan of the active run")
	return cmd
}

func advisorLine(report domain.Report, models []string) string {
	switch {
	case report.Track == shareddomain.TrackFull:
		return advisorOnLine(models, 0)
	case domain.NeedsAdvisor(report.Doc):
		return advisorOnLine(models, 1) + advisorPath
	default:
		return advisorOff
	}
}

func advisorOnLine(models []string, index int) string {
	switch {
	case len(models) == 0:
		return advisorOnBare
	case index >= len(models):
		index = len(models) - 1
	}
	return fmt.Sprintf(advisorOn, models[index])
}

// waveLines gives the lead the task order as text, so that it reads no plan page.
func waveLines(tasks []domain.Task) []string {
	waves, err := domain.Waves(tasks)
	if err != nil {
		return nil
	}
	models := make(map[int]string, len(tasks))
	for _, t := range tasks {
		models[t.Number] = t.Model
	}
	lines := make([]string, len(waves))
	for i, wave := range waves {
		names := make([]string, len(wave))
		for j, n := range wave {
			names[j] = fmt.Sprintf(waveTaskForm, n)
			if models[n] != "" {
				names[j] = fmt.Sprintf(waveModelForm, n, models[n])
			}
		}
		lines[i] = fmt.Sprintf(waveLineForm, i+1, strings.Join(names, waveTaskSep))
	}
	return lines
}

func queryFor(cmd *cobra.Command, handlers Handlers) (checkplan.Query, error) {
	if path, _ := cmd.Flags().GetString(flagFile); path != "" {
		return checkplan.Query{Path: path}, nil
	}
	return handlers.DefaultPlan(cmd.Context())
}
