package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/run/application/command/start"
	"github.com/90p-labs/harness/internal/run/application/command/transition"
	"github.com/90p-labs/harness/internal/run/application/query/report"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	commandRun    = "run"
	commandStart  = "start"
	commandStatus = "status"

	flagFindings = "findings"
	flagReason   = "reason"
	flagPR       = "pr"
	flagBy       = "by"
	flagClass    = "class"

	runLineFormat  = "run %s phase=%s review_round=%d live_round=%d"
	reasonPrefix   = "reason: "
	nextPrefix     = "next: "
	noActiveRun    = "active: no"
	missingFlagMsg = "required flag %q not set"
	pairedFlagsMsg = "flags %q and %q go together"
	unknownClass   = "unknown rework class %q"
	planningHint   = "write the plan in %s, then harness run plan"
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
	domain.PhaseShipping:         "merge the change, then harness run done --pr URL",
	domain.PhaseReleasing:        "deploy the change and wait for the live check",
	domain.PhaseObserving:        "watch the signals until the live check ends the run",
	domain.PhaseBlocked:          "fix the cause, then harness run resume, or harness run abandon",
	domain.PhaseDone:             "the run is closed, start the next with harness run start",
	domain.PhaseAbandoned:        "the run is closed, start the next with harness run start",
}

type Handlers struct {
	Key        domain.RunKey
	Start      start.Handler
	Transition transition.Handler
	Status     status.Handler
	Report     report.Handler
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
	{"amend", "Reopen the plan", domain.EventAmend, ""},
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
	cmd.AddCommand(startCommand(connect), statusCommand(connect), reportCommand(connect))
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
	return append(lines, nextPrefix+hint(s))
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
				if err := h.Start.Handle(ctx, start.Command{Key: h.Key, Prompt: args[0]}); err != nil {
					return err
				}
				if err := printStatus(ctx, cmd.OutOrStdout(), h, status.Query{Key: h.Key}); err != nil {
					return err
				}
				return printRepeatClasses(ctx, cmd.OutOrStdout(), cmd.ErrOrStderr(), h)
			})
		},
	}
}

func statusCommand(connect Connect) *cobra.Command {
	return &cobra.Command{
		Use:   commandStatus,
		Short: "Print the active run",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				err := printStatus(ctx, cmd.OutOrStdout(), h, status.Query{Key: h.Key})
				if errors.Is(err, domain.ErrRunNotFound) {
					_, err = fmt.Fprintln(cmd.OutOrStdout(), noActiveRun)
				}
				return err
			})
		},
	}
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
				return printStatus(ctx, cmd.OutOrStdout(), h, status.Query{RunID: current.ID})
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

func printStatus(ctx context.Context, out io.Writer, h Handlers, query status.Query) error {
	current, err := h.Status.Handle(ctx, query)
	if err != nil {
		return err
	}
	for _, line := range Lines(current) {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	return nil
}
