package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/jorge07/harness/internal/gate/application/command/verify"
	"github.com/jorge07/harness/internal/gate/application/command/verifycommits"
	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/gate/ui/cli"
)

const (
	commitsBase = "main"
	failedTail  = "FAIL: TestLogin\nexit status 1"
)

func connectWith(handlers cli.Handlers, closed *bool) cli.Connect {
	return func(context.Context) (cli.Handlers, func() error, error) {
		return handlers, func() error { *closed = true; return nil }, nil
	}
}

func run(t *testing.T, cmd *cobra.Command, args ...string) (stdout string, err error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err = cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestVerifyCommand(t *testing.T) {
	passing := verify.Result{Lines: []verify.Line{
		{Gate: "vet", Status: domain.StatusPass, Duration: 1500 * time.Millisecond},
		{Gate: "test", Status: domain.StatusCached, Duration: 20 * time.Second},
	}}
	failing := verify.Result{Failed: true, Lines: []verify.Line{
		{Gate: "vet", Status: domain.StatusPass, Duration: time.Second},
		{Gate: "test", Status: domain.StatusFail, Duration: 2 * time.Second, Tail: failedTail},
		{Gate: "criterion-3", Status: domain.StatusVetoed, Tail: "not now"},
	}}

	t.Run("all pass", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			Verify: func(context.Context) (verify.Result, error) { return passing, nil },
		}, &closed))

		stdout, err := run(t, cmd)

		if err != nil || stdout != "PASS vet 1.5s\nCACHED test 20s\n" || !closed {
			t.Fatalf("err = %v, stdout = %q, closed = %v", err, stdout, closed)
		}
	})

	t.Run("one fails", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			Verify: func(context.Context) (verify.Result, error) { return failing, nil },
		}, &closed))

		stdout, err := run(t, cmd)

		want := "PASS vet 1s\nFAIL test 2s\nVETOED criterion-3 0s\n\ntest:\n" + failedTail + "\n\ncriterion-3:\nnot now\n"
		if !errors.Is(err, cli.ErrGatesFailed) || stdout != want {
			t.Fatalf("err = %v, stdout = %q, want ErrGatesFailed and %q", err, stdout, want)
		}
	})

	t.Run("the setup steps that ran print before the gates", func(t *testing.T) {
		var closed bool
		withSetup := passing
		withSetup.Setup = []verify.SetupLine{{Step: "deps", Duration: 3200 * time.Millisecond}}
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			Verify: func(context.Context) (verify.Result, error) { return withSetup, nil },
		}, &closed))

		stdout, err := run(t, cmd)

		if err != nil || stdout != "setup deps 3.2s\nPASS vet 1.5s\nCACHED test 20s\n" {
			t.Fatalf("err = %v, stdout = %q", err, stdout)
		}
	})

	t.Run("a failed need skips the gates after it", func(t *testing.T) {
		var closed bool
		skipped := verify.Result{Failed: true, Lines: []verify.Line{
			{Gate: "lint", Status: domain.StatusFail, Duration: time.Second, Tail: failedTail},
			{Gate: "test", Status: domain.StatusSkipped, Need: "lint"},
			{Gate: "e2e", Status: domain.StatusSkipped, Need: "test"},
		}}
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			Verify: func(context.Context) (verify.Result, error) { return skipped, nil },
		}, &closed))

		stdout, err := run(t, cmd)

		want := "FAIL lint 1s\nSKIPPED test needs lint\nSKIPPED e2e needs test\n\nlint:\n" + failedTail + "\n"
		if !errors.Is(err, cli.ErrGatesFailed) || stdout != want {
			t.Fatalf("err = %v, stdout = %q, want ErrGatesFailed and %q", err, stdout, want)
		}
	})

	t.Run("no gate is set", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			Verify: func(context.Context) (verify.Result, error) { return verify.Result{}, nil },
		}, &closed))

		stdout, err := run(t, cmd)

		if err != nil || !strings.Contains(stdout, "no gate is set") {
			t.Fatalf("err = %v, stdout = %q, want the no gate message", err, stdout)
		}
	})

	t.Run("a handler error", func(t *testing.T) {
		var closed bool
		boom := errors.New("git is gone")
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			Verify: func(context.Context) (verify.Result, error) { return verify.Result{}, boom },
		}, &closed))

		stdout, err := run(t, cmd)

		if !errors.Is(err, boom) || stdout != "" || !closed {
			t.Fatalf("err = %v, stdout = %q, closed = %v", err, stdout, closed)
		}
	})

	t.Run("commits", func(t *testing.T) {
		var closed bool
		var gotBase string
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			VerifyCommits: func(_ context.Context, base string) (verifycommits.Result, error) {
				gotBase = base
				return verifycommits.Result{Commits: []domain.CommitResult{
					{SHA: "0123456789abcdef", Type: "test", Passed: false},
					{SHA: "fedcba9876543210", Type: "feat", Passed: true},
				}}, nil
			},
		}, &closed))

		stdout, err := run(t, cmd, "--commits", commitsBase)

		if err != nil || gotBase != commitsBase || stdout != "FAIL 0123456789ab test\nPASS fedcba987654 feat\n" {
			t.Fatalf("err = %v, base = %q, stdout = %q", err, gotBase, stdout)
		}
	})

	t.Run("a commit rule breaks", func(t *testing.T) {
		var closed bool
		broken := errors.Join(domain.ErrCommitRule)
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			VerifyCommits: func(context.Context, string) (verifycommits.Result, error) {
				return verifycommits.Result{Commits: []domain.CommitResult{{SHA: "0123456789abcdef", Type: "feat", Passed: false}}}, broken
			},
		}, &closed))

		stdout, err := run(t, cmd, "--commits", commitsBase)

		if !errors.Is(err, domain.ErrCommitRule) || stdout != "FAIL 0123456789ab feat\n" {
			t.Fatalf("err = %v, stdout = %q, want the commit line and the rule error", err, stdout)
		}
	})
}
