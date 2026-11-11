package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/gate/application/command/initgates"
	"github.com/90p-labs/harness/internal/gate/application/command/verify"
	"github.com/90p-labs/harness/internal/gate/application/command/verifycommits"
	"github.com/90p-labs/harness/internal/gate/domain"
)

const (
	commandVerify = "verify"
	flagCommits   = "commits"

	lineFormat        = "%s %s %s\n"
	skipFormat        = "%s %s needs %s\n"
	setupFormat       = "setup %s %s\n"
	tailFormat        = "\n%s:\n%s\n"
	noGateMessage     = "no gate is set\n"
	passGatesFormat   = "%s gates=%d cached=%d\n"
	failGatesFormat   = "%s gates=%d failed=%d\n"
	passCommitsFormat = "%s commits=%d\n"
	baseFormat        = "base %s %s\n"
	failedGateFormat  = "  %s %s %s\n"
	tailLabelFormat   = "%s %s"
	shortSHALength    = 12
)

var ErrGatesFailed = errors.New("gates failed")

type Handlers struct {
	Verify        func(ctx context.Context) (verify.Result, error)
	VerifyCommits func(ctx context.Context, base string) (verifycommits.Result, error)
	Init          func(ctx context.Context) (initgates.Result, error)
}

type Connect func(ctx context.Context) (Handlers, func() error, error)

func NewVerifyCommand(connect Connect) *cobra.Command {
	cmd := &cobra.Command{
		Use:   commandVerify,
		Short: "Run the gates and the plan criteria",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			base, _ := cmd.Flags().GetString(flagCommits)
			return withHandlers(cmd, connect, func(ctx context.Context, h Handlers) error {
				if base != "" {
					return verifyCommits(ctx, cmd.OutOrStdout(), h, base)
				}
				return verifyGates(ctx, cmd.OutOrStdout(), h)
			})
		},
	}
	cmd.Flags().String(flagCommits, "", "check each commit from the merge base of BASE, or of origin/BASE when newer, to HEAD")
	return cmd
}

func verifyGates(ctx context.Context, out io.Writer, h Handlers) error {
	result, err := h.Verify(ctx)
	if err != nil {
		return err
	}
	for _, step := range result.Setup {
		if _, err := fmt.Fprintf(out, setupFormat, step.Step, step.Duration.Round(time.Millisecond)); err != nil {
			return err
		}
	}
	if len(result.Lines) == 0 {
		_, err = fmt.Fprint(out, noGateMessage)
		return err
	}
	failed, cached := 0, 0
	for _, line := range result.Lines {
		var err error
		switch {
		case line.Status == domain.StatusCached:
			cached++
		case line.Status == domain.StatusSkipped:
			_, err = fmt.Fprintf(out, skipFormat, line.Status, line.Gate, line.Need)
		case isFailedStatus(line.Status):
			failed++
			_, err = fmt.Fprintf(out, lineFormat, line.Status, line.Gate, line.Duration.Round(time.Millisecond))
		}
		if err != nil {
			return err
		}
	}
	for _, line := range result.Lines {
		if !isFailedStatus(line.Status) {
			continue
		}
		if _, err := fmt.Fprintf(out, tailFormat, line.Gate, line.Tail); err != nil {
			return err
		}
	}
	if !result.Failed {
		_, err = fmt.Fprintf(out, passGatesFormat, domain.StatusPass, len(result.Lines), cached)
	} else {
		_, err = fmt.Fprintf(out, failGatesFormat, domain.StatusFail, len(result.Lines), failed)
	}
	if err != nil {
		return err
	}
	if result.Failed {
		return ErrGatesFailed
	}
	return nil
}

func isFailedStatus(status domain.Status) bool {
	return status == domain.StatusFail || status == domain.StatusVetoed
}

func verifyCommits(ctx context.Context, out io.Writer, h Handlers, base string) error {
	result, err := h.VerifyCommits(ctx, base)
	if err != nil && !errors.Is(err, domain.ErrCommitRule) {
		return err
	}
	if result.Base.Ref != "" {
		if _, werr := fmt.Fprintf(out, baseFormat, result.Base.Ref, shortSHA(result.Base.SHA)); werr != nil {
			return werr
		}
	}
	if err == nil {
		_, werr := fmt.Fprintf(out, passCommitsFormat, domain.StatusPass, len(result.Commits))
		return werr
	}
	for _, commit := range result.Commits {
		status := domain.StatusFail
		if commit.Passed {
			status = domain.StatusPass
		}
		if _, werr := fmt.Fprintf(out, lineFormat, status, shortSHA(commit.SHA), commit.Type); werr != nil {
			return werr
		}
		for _, failed := range commit.Failed {
			if _, werr := fmt.Fprintf(out, failedGateFormat, failed.Status, failed.Gate, failed.Duration.Round(time.Millisecond)); werr != nil {
				return werr
			}
		}
	}
	for _, commit := range result.Commits {
		for _, failed := range commit.Failed {
			label := fmt.Sprintf(tailLabelFormat, shortSHA(commit.SHA), failed.Gate)
			if _, werr := fmt.Fprintf(out, tailFormat, label, failed.Tail); werr != nil {
				return werr
			}
		}
	}
	return err
}

func shortSHA(sha string) string {
	if len(sha) > shortSHALength {
		return sha[:shortSHALength]
	}
	return sha
}

func withHandlers(cmd *cobra.Command, connect Connect, run func(context.Context, Handlers) error) (err error) {
	handlers, closeStore, err := connect(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeStore()) }()
	return run(cmd.Context(), handlers)
}
