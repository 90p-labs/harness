package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/run/application/command/start"
	"github.com/90p-labs/harness/internal/run/application/command/transition"
	"github.com/90p-labs/harness/internal/run/application/command/waitpr"
	"github.com/90p-labs/harness/internal/run/application/query/learn"
	"github.com/90p-labs/harness/internal/run/application/query/report"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	commandRun    = "run"
	commandStart  = "start"
	commandStatus = "status"
	commandWait   = "wait"

	flagFindings = "findings"
	flagReason   = "reason"
	flagPR       = "pr"
	flagBy       = "by"
	flagClass    = "class"

	runLineFormat      = "run %s phase=%s review_round=%d live_round=%d"
	reasonPrefix       = "reason: "
	nextPrefix         = "next: "
	handoffReadyFormat = "handoff: ready (step %d)"
	handoffWriteFormat = `handoff: write %s with Phase: %s, Step: %d and the sections %s, each with a "- " line, then continue`
	noActiveRun        = "active: no"
	gitignoreAddedLine = "gitignore: added " + shareddomain.RunsFolder + "/"
	missingFlagMsg     = "required flag %q not set"
	pairedFlagsMsg     = "flags %q and %q go together"
	unknownClass       = "unknown rework class %q"
	planningHint       = "write the plan in %s, then harness run plan"
)

var reworkClasses = []domain.ReworkClass{
	domain.ReworkImplementation, domain.ReworkSpecGap, domain.ReworkGateCheckable, domain.ReworkMinor,
}

func reworkClassNames(separator, lastSeparator string) string {
	names := make([]string, len(reworkClasses))
	for i, class := range reworkClasses {
		names[i] = string(class)
	}
	last := len(names) - 1
	return strings.Join(names[:last], separator) + lastSeparator + names[last]
}

var phaseHints = map[domain.Phase]string{
	domain.PhaseAwaitingApproval: "ask the Plan question; when the user picks Approve, run harness run approve",
	domain.PhaseWorking:          "implement test first, run harness verify, then harness run review",
	domain.PhaseReviewing:        "review the change, then harness run ship, or harness run rework --findings \"<id, severity, file:line>\" --class <" + reworkClassNames("|", "|") + ">",
	domain.PhaseShipping:         "run harness run learn, open the pull request and run harness run wait --pr URL, then after the merge harness run done --pr URL",
	domain.PhaseReleasing:        "deploy the change and wait for the live check",
	domain.PhaseObserving:        "watch the signals until the live check ends the run",
	domain.PhaseBlocked:          "fix the cause, then harness run resume, or harness run abandon",
	domain.PhaseDone:             "the run is closed, start the next with harness run start",
	domain.PhaseAbandoned:        "the run is closed, start the next with harness run start",
}

// HostModels is the resolved role models of one host, one entry per line of the models output.
type HostModels struct {
	Host  string
	Roles map[string][]string
}

const (
	modelsLabel      = "models"
	modelsListJoiner = ","
)

var modelRoles = []shareddomain.Role{
	shareddomain.RoleLead, shareddomain.RoleExplorer, shareddomain.RoleWorker,
	shareddomain.RoleReviewer, shareddomain.RoleAdvisor,
}

// modelLines prints one line per host that has roles and names the host only when more than one line prints.
func modelLines(all []HostModels) []string {
	models := make([]HostModels, 0, len(all))
	for _, m := range all {
		if len(m.Roles) > 0 {
			models = append(models, m)
		}
	}
	lines := make([]string, 0, len(models))
	for _, m := range models {
		parts := make([]string, 0, len(modelRoles))
		for _, role := range modelRoles {
			parts = append(parts, fmt.Sprintf("%s=%s", role, strings.Join(m.Roles[string(role)], modelsListJoiner)))
		}
		label := modelsLabel
		if len(models) > 1 {
			label = fmt.Sprintf("%s (%s)", modelsLabel, m.Host)
		}
		lines = append(lines, label+": "+strings.Join(parts, " "))
	}
	return lines
}

type Handlers struct {
	Models     []HostModels
	IgnoreRuns func() (bool, error)
	Key        domain.RunKey
	Start      start.Handler
	Transition transition.Handler
	WaitPR     waitpr.Handler
	Status     status.Handler
	Report     report.Handler
	Learn      learn.Handler
	// AfterStart runs after a run starts and returns one line to print. An
	// error only prints a warning and never fails the start. Nil skips it.
	AfterStart func(ctx context.Context) (string, error)
}

type Connect func(ctx context.Context) (Handlers, func() error, error)

type transitionSpec struct {
	use   string
	short string
	event domain.Event
	flag  string
}

var transitionSpecs = []transitionSpec{
	{"plan", "Submit the plan", domain.EventSubmitPlan, ""},
	{"approve", "Approve the plan", domain.EventApprovePlan, ""},
	{"amend", "Amend the approved plan", domain.EventAmend, ""},
	{"review", "Submit the change for review", domain.EventSubmitForReview, ""},
	{"rework", "Send the change back with findings", domain.EventRequestRework, flagFindings},
	{"ship", "Approve the review", domain.EventApproveReview, ""},
	{"done", "Complete the run", domain.EventComplete, flagPR},
	{"block", "Block the run", domain.EventBlock, flagReason},
	{"resume", "Resume a blocked run", domain.EventResume, ""},
	{"abandon", "Abandon the run", domain.EventAbandon, ""},
}

func NewCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{Use: commandRun, Short: "Drive the active run"}
	cmd.AddCommand(startCommand(connect), statusCommand(connect), reportCommand(connect), learnCommand(connect), waitCommand(connect))
	for _, spec := range transitionSpecs {
		cmd.AddCommand(transitionCommand(connect, spec))
	}
	return cmd
}

func Lines(s status.Status) []string {
	lines := []string{fmt.Sprintf(runLineFormat, s.ID, s.Phase, s.ReviewRound, s.LiveRound)}
	if s.Reason != "" {
		lines = append(lines, reasonPrefix+s.Reason)
	}
	if s.Step > 0 && s.HandoffStep == s.Step {
		lines = append(lines, fmt.Sprintf(handoffReadyFormat, s.Step))
	}
	return append(lines, nextPrefix+hint(s))
}

func HandoffWriteLine(s status.Status) string {
	if !s.Phase.HoldsTheTurn() {
		return ""
	}
	return fmt.Sprintf(handoffWriteFormat, shareddomain.RunHandoffFile(s.ID), s.Phase, s.Step, strings.Join(domain.HandoffSections(), ", "))
}

func hint(s status.Status) string {
	if s.Phase == domain.PhasePlanning {
		return fmt.Sprintf(planningHint, shareddomain.RunPlanFile(s.ID))
	}
	return phaseHints[s.Phase]
}

func startCommand(connect Connect) *cobra.Command {
	return &cobra.Command{
		Use:   commandStart + " <prompt>",
		Short: "Start a run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				lines := modelLines(h.Models)
				added := false
				if h.IgnoreRuns != nil {
					var err error
					if added, err = h.IgnoreRuns(); err != nil {
						return err
					}
				}
				if added {
					lines = append(lines, gitignoreAddedLine)
				}
				// reportAdded prints the gitignore line when the start fails, so no change to the file goes unreported.
				reportAdded := func(err error) error {
					if !added {
						return err
					}
					_, writeErr := fmt.Fprintln(cmd.OutOrStdout(), gitignoreAddedLine)
					return errors.Join(err, writeErr)
				}
				if err := h.Start.Handle(ctx, start.Command{Key: h.Key, Prompt: args[0]}); err != nil {
					return reportAdded(err)
				}
				current, err := h.Status.Handle(ctx, status.Query{Key: h.Key})
				if err != nil {
					return reportAdded(err)
				}
				if err := writeStatus(cmd.OutOrStdout(), current, lines); err != nil {
					return err
				}
				if err := runAfterStart(ctx, cmd, h); err != nil {
					return err
				}
				return printRepeatClasses(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), h)
			})
		},
	}
}

// runAfterStart prints the line of the autostart, or a warning. Only a failed
// write fails the command.
func runAfterStart(ctx context.Context, cmd *cobra.Command, h Handlers) error {
	if h.AfterStart == nil {
		return nil
	}
	line, err := h.AfterStart(ctx)
	if err != nil {
		_, writeErr := fmt.Fprintf(cmd.ErrOrStderr(), "panel: start failed: %v\n", err)
		return writeErr
	}
	if line == "" {
		return nil
	}
	_, err = fmt.Fprintln(cmd.OutOrStdout(), line)
	return err
}

func statusCommand(connect Connect) *cobra.Command {
	return &cobra.Command{
		Use:   commandStatus,
		Short: "Print the active run",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				_, err := printStatus(ctx, cmd.OutOrStdout(), h, status.Query{Key: h.Key}, modelLines(h.Models))
				if errors.Is(err, domain.ErrRunNotFound) {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), noActiveRun)
				}
				return err
			})
		},
	}
}

func waitCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandWait,
		Short: "Wait on the open pull request",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			if flagValue(cmd, flagPR) == "" {
				return fmt.Errorf(missingFlagMsg, flagPR)
			}
			_, err := shareddomain.ParsePRURL(flagValue(cmd, flagPR))
			return err
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				if err := h.WaitPR.Handle(ctx, waitpr.Command{Key: h.Key, PRURL: flagValue(cmd, flagPR)}); err != nil {
					return err
				}
				_, err := printStatus(ctx, cmd.OutOrStdout(), h, status.Query{Key: h.Key}, nil)
				return err
			})
		},
	}
	cmd.Flags().String(flagPR, "", "the "+flagPR+" of the change")
	return cmd
}

func transitionCommand(connect Connect, spec transitionSpec) *cobra.Command {
	approvedBy := spec.event == domain.EventApprovePlan
	classed := spec.event == domain.EventRequestRework
	cmd := &cobra.Command{
		Use:   spec.use,
		Short: spec.short,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.NoArgs(cmd, args); err != nil {
				return err
			}
			if spec.flag != "" && flagValue(cmd, spec.flag) == "" {
				return fmt.Errorf(missingFlagMsg, spec.flag)
			}
			if approvedBy && (flagValue(cmd, flagBy) == "") != (flagValue(cmd, flagReason) == "") {
				return fmt.Errorf(pairedFlagsMsg, flagBy, flagReason)
			}
			if class := domain.ReworkClass(flagValue(cmd, flagClass)); classed && class != "" && !class.Known() {
				return fmt.Errorf(unknownClass, class)
			}
			if spec.flag == flagPR {
				_, err := shareddomain.ParsePRURL(flagValue(cmd, flagPR))
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				current, err := h.Status.Handle(ctx, status.Query{Key: h.Key})
				if err != nil {
					return err
				}
				command := transition.Command{Key: h.Key, Event: spec.event}
				switch {
				case spec.flag == flagPR:
					command.PRURL = flagValue(cmd, flagPR)
				case approvedBy:
					command.By, command.Reason = flagValue(cmd, flagBy), flagValue(cmd, flagReason)
				default:
					command.Reason = flagValue(cmd, spec.flag)
				}
				command.Class = domain.ReworkClass(flagValue(cmd, flagClass))
				if err = h.Transition.Handle(ctx, command); err != nil {
					return err
				}
				after, err := printStatus(ctx, cmd.OutOrStdout(), h, status.Query{RunID: current.ID}, nil)
				if err != nil {
					return err
				}
				if line := HandoffWriteLine(after); line != "" {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), line)
				}
				return err
			})
		},
	}
	if spec.flag != "" {
		cmd.Flags().String(spec.flag, "", "the "+spec.flag+" of the change")
	}
	if classed {
		cmd.Flags().String(flagClass, "", "the class of the rework: "+reworkClassNames(", ", " or "))
	}
	if approvedBy {
		cmd.Flags().String(flagBy, "", "the model that approves without the recorded answer of the user")
		cmd.Flags().String(flagReason, "", "why the model approves, with --by")
	}
	return cmd
}

func flagValue(cmd *cobra.Command, name string) string {
	if name == "" {
		return ""
	}
	value, _ := cmd.Flags().GetString(name)
	return value
}

func withHandlers(cmd *cobra.Command, connect Connect, run func(context.Context, Handlers) error) (err error) {
	handlers, closeStore, err := connect(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeStore()) }()
	return run(cmd.Context(), handlers)
}

// printStatus prints the status lines; extra lines go after the first line.
func printStatus(ctx context.Context, out io.Writer, h Handlers, query status.Query, extra []string) (status.Status, error) {
	current, err := h.Status.Handle(ctx, query)
	if err != nil {
		return current, err
	}
	return current, writeStatus(out, current, extra)
}

// writeStatus writes the status lines of current; extra lines go after the first line.
func writeStatus(out io.Writer, current status.Status, extra []string) error {
	lines := Lines(current)
	if len(extra) > 0 {
		lines = slices.Concat(lines[:1], extra, lines[1:])
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	return nil
}
