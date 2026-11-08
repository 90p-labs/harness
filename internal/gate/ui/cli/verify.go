package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/gate/application/command/verify"
	"github.com/jorge07/harness/internal/gate/application/command/verifycommits"
	"github.com/jorge07/harness/internal/gate/domain"
)

const (
	commandVerify = "verify"
	flagCommits   = "commits"

	lineFormat     = "%s %s %s\n"
	skipFormat     = "%s %s needs %s\n"
	tailFormat     = "\n%s:\n%s\n"
	noGateMessage  = "no gate is set\n"
	shortSHALength = 12
)

var ErrGatesFailed = errors.New("gates failed")

type Handlers struct {
	Verify        func(ctx context.Context) (verify.Result, error)
	VerifyCommits func(ctx context.Context, base string) (verifycommits.Result, error)
	Init          func(ctx context.Context) ([]domain.Gate, error)
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
	cmd.Flags().String(flagCommits, "", "check each commit of BASE..HEAD")
	return cmd
}

func verifyGates(ctx context.Context, out io.Writer, h Handlers) error {
	result, err := h.Verify(ctx)
	if err != nil {
		return err
	}
	if len(result.Lines) == 0 {
		_, err = fmt.Fprint(out, noGateMessage)
		return err
	}
	for _, line := range result.Lines {
		var err error
		if line.Status == domain.StatusSkipped {
			_, err = fmt.Fprintf(out, skipFormat, line.Status, line.Gate, line.Need)
		} else {
			_, err = fmt.Fprintf(out, lineFormat, line.Status, line.Gate, line.Duration.Round(time.Millisecond))
		}
		if err != nil {
			return err
		}
	}
	for _, line := range result.Lines {
		if line.Status != domain.StatusFail && line.Status != domain.StatusVetoed {
			continue
		}
		if _, err := fmt.Fprintf(out, tailFormat, line.Gate, line.Tail); err != nil {
			return err
		}
	}
	if result.Failed {
		return ErrGatesFailed
	}
	return nil
}

func verifyCommits(ctx context.Context, out io.Writer, h Handlers, base string) error {
	result, err := h.VerifyCommits(ctx, base)
	if err != nil && !errors.Is(err, domain.ErrCommitRule) {
		return err
	}
	for _, commit := range result.Commits {
		status := domain.StatusFail
		if commit.Passed {
			status = domain.StatusPass
		}
		sha := commit.SHA
		if len(sha) > shortSHALength {
			sha = sha[:shortSHALength]
		}
		if _, werr := fmt.Fprintf(out, lineFormat, status, sha, commit.Type); werr != nil {
			return werr
		}
	}
	return err
}

func withHandlers(cmd *cobra.Command, connect Connect, run func(context.Context, Handlers) error) (err error) {
	handlers, closeStore, err := connect(cmd.Context())
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, closeStore()) }()
	return run(cmd.Context(), handlers)
}
