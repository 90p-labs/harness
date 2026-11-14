package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/90p-labs/harness/internal/gate/application/command/verify"
	"github.com/90p-labs/harness/internal/gate/application/command/verifycommits"
	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/gate/ui/cli"
)

const (
	commitsBase = "main"
	failedTail  = "FAIL: TestLogin\nexit status 1"
)

var stubBase = domain.Base{Ref: "origin/main", SHA: "0a83514a6119abcdef0123456789abcdef012345"}

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

		if err != nil || stdout != "PASS gates=2 cached=1\n" || !closed {
			t.Fatalf("err = %v, stdout = %q, closed = %v", err, stdout, closed)
		}
	})

	t.Run("one fails", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			Verify: func(context.Context) (verify.Result, error) { return failing, nil },
		}, &closed))

		stdout, err := run(t, cmd)

		want := "FAIL test 2s\nVETOED criterion-3 0s\n\ntest:\n" + failedTail + "\n\ncriterion-3:\nnot now\nFAIL gates=3 failed=2\n"
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

		if err != nil || stdout != "setup deps 3.2s\nPASS gates=2 cached=1\n" {
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

		want := "FAIL lint 1s\nSKIPPED test needs lint\nSKIPPED e2e needs test\n\nlint:\n" + failedTail + "\nFAIL gates=3 failed=1\n"
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

		if err != nil || gotBase != commitsBase || stdout != "PASS commits=2\n" {
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

func TestVerifyShortOutputPass(t *testing.T) {
	var closed bool
	cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
		Verify: func(context.Context) (verify.Result, error) {
			return verify.Result{
				Setup: []verify.SetupLine{{Step: "deps", Duration: time.Second}},
				Lines: []verify.Line{
					{Gate: "vet", Status: domain.StatusPass, Duration: time.Second},
					{Gate: "test", Status: domain.StatusCached},
					{Gate: "lint", Status: domain.StatusCached},
				},
			}, nil
		},
	}, &closed))

	stdout, err := run(t, cmd)

	if err != nil || stdout != "setup deps 1s\nPASS gates=3 cached=2\n" {
		t.Fatalf("err = %v, stdout = %q", err, stdout)
	}
}

func TestVerifyShortOutputFail(t *testing.T) {
	var closed bool
	cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
		Verify: func(context.Context) (verify.Result, error) {
			return verify.Result{Failed: true, Lines: []verify.Line{
				{Gate: "vet", Status: domain.StatusPass, Duration: time.Second},
				{Gate: "lint", Status: domain.StatusCached},
				{Gate: "test", Status: domain.StatusFail, Duration: 2 * time.Second, Tail: failedTail},
				{Gate: "criterion-3", Status: domain.StatusVetoed, Tail: "not now"},
				{Gate: "e2e", Status: domain.StatusSkipped, Need: "test"},
			}}, nil
		},
	}, &closed))

	stdout, err := run(t, cmd)

	want := "FAIL test 2s\nVETOED criterion-3 0s\nSKIPPED e2e needs test\n\ntest:\n" + failedTail + "\n\ncriterion-3:\nnot now\nFAIL gates=5 failed=2\n"
	if !errors.Is(err, cli.ErrGatesFailed) || stdout != want {
		t.Fatalf("err = %v, stdout = %q, want ErrGatesFailed and %q", err, stdout, want)
	}
}

func TestVerifyCommitsShortOutput(t *testing.T) {
	t.Run("the rule holds with a failing test commit", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			VerifyCommits: func(context.Context, string) (verifycommits.Result, error) {
				return verifycommits.Result{Base: stubBase, Commits: []domain.CommitResult{
					{SHA: "0123456789abcdef", Type: "test", Passed: false},
					{SHA: "fedcba9876543210", Type: "feat", Passed: true},
				}}, nil
			},
		}, &closed))

		stdout, err := run(t, cmd, "--commits", commitsBase)

		if err != nil || stdout != "base origin/main 0a83514a6119\nPASS commits=2\n" {
			t.Fatalf("err = %v, stdout = %q", err, stdout)
		}
	})

	t.Run("a rule error prints each commit", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			VerifyCommits: func(context.Context, string) (verifycommits.Result, error) {
				return verifycommits.Result{Base: stubBase, Commits: []domain.CommitResult{
					{SHA: "0123456789abcdef", Type: "feat", Passed: false},
					{SHA: "fedcba9876543210", Type: "fix", Passed: true},
				}}, domain.ErrCommitRule
			},
		}, &closed))

		stdout, err := run(t, cmd, "--commits", commitsBase)

		want := "base origin/main 0a83514a6119\nFAIL 0123456789ab feat\nPASS fedcba987654 fix\n"
		if !errors.Is(err, domain.ErrCommitRule) || stdout != want {
			t.Fatalf("err = %v, stdout = %q", err, stdout)
		}
	})

	t.Run("a rule error prints the failed gates and their tails", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			VerifyCommits: func(context.Context, string) (verifycommits.Result, error) {
				return verifycommits.Result{Base: stubBase, Commits: []domain.CommitResult{
					{SHA: "006bdb343baf1234", Type: "feat", Passed: false, Failed: []domain.GateFailure{
						{Gate: "test", Status: domain.StatusFail, Duration: 12*time.Millisecond + 400*time.Microsecond, Tail: failedTail},
						{Gate: "lint", Status: domain.StatusVetoed, Duration: 3 * time.Millisecond, Tail: "vetoed"},
					}},
					{SHA: "e2026fc109971234", Type: "fix", Passed: true},
				}}, domain.ErrCommitRule
			},
		}, &closed))

		stdout, err := run(t, cmd, "--commits", commitsBase)

		want := "base origin/main 0a83514a6119\n" +
			"FAIL 006bdb343baf feat\n" +
			"  FAIL test 12ms\n" +
			"  VETOED lint 3ms\n" +
			"PASS e2026fc10997 fix\n" +
			"\n006bdb343baf test:\n" + failedTail + "\n" +
			"\n006bdb343baf lint:\nvetoed\n"
		if !errors.Is(err, domain.ErrCommitRule) || stdout != want {
			t.Fatalf("err = %v, stdout = %q, want %q", err, stdout, want)
		}
	})
}

func TestVerifyOneCriterionFlag(t *testing.T) {
	t.Run("the flag passes its number to the handler", func(t *testing.T) {
		var closed bool
		got := 0
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			VerifyCriterion: func(_ context.Context, n int) (verify.Result, error) {
				got = n
				return verify.Result{Lines: []verify.Line{{Gate: "criterion-2", Status: domain.StatusPass}}}, nil
			},
		}, &closed))

		stdout, err := run(t, cmd, "--criterion", "2")

		if err != nil || got != 2 || stdout != "PASS gates=1 cached=0\n" || !closed {
			t.Fatalf("err = %v, n = %d, stdout = %q, closed = %v", err, got, stdout, closed)
		}
	})

	t.Run("a failed proof prints its tail and the error", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
			VerifyCriterion: func(context.Context, int) (verify.Result, error) {
				return verify.Result{Failed: true, Lines: []verify.Line{
					{Gate: "criterion-2", Status: domain.StatusFail, Duration: time.Second, Tail: failedTail},
				}}, nil
			},
		}, &closed))

		stdout, err := run(t, cmd, "--criterion", "2")

		want := "FAIL criterion-2 1s\n\ncriterion-2:\n" + failedTail + "\nFAIL gates=1 failed=1\n"
		if !errors.Is(err, cli.ErrGatesFailed) || stdout != want {
			t.Fatalf("err = %v, stdout = %q, want ErrGatesFailed and %q", err, stdout, want)
		}
	})

	t.Run("the flag with --commits is a flag error before any connection", func(t *testing.T) {
		var closed bool
		cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{}, &closed))
		root := &cobra.Command{Use: "harness"}
		var flagErr error
		root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { flagErr = err; return err })
		root.AddCommand(cmd)

		_, err := run(t, root, "verify", "--criterion", "2", "--commits", commitsBase)

		if err == nil || flagErr == nil || closed {
			t.Fatalf("err = %v, flag error = %v, closed = %v, want a flag error before any connection", err, flagErr, closed)
		}
	})
}

func TestVerifyOneCriterionRunsOneGate(t *testing.T) {
	var closed bool
	var calls []string
	cmd := cli.NewVerifyCommand(connectWith(cli.Handlers{
		Verify: func(context.Context) (verify.Result, error) {
			calls = append(calls, "verify")
			return verify.Result{}, nil
		},
		VerifyCommits: func(context.Context, string) (verifycommits.Result, error) {
			calls = append(calls, "commits")
			return verifycommits.Result{}, nil
		},
		VerifyCriterion: func(context.Context, int) (verify.Result, error) {
			calls = append(calls, "criterion")
			return verify.Result{Lines: []verify.Line{{Gate: "criterion-1", Status: domain.StatusCached}}}, nil
		},
	}, &closed))

	stdout, err := run(t, cmd, "--criterion", "1")

	if err != nil || len(calls) != 1 || calls[0] != "criterion" || stdout != "PASS gates=1 cached=1\n" {
		t.Fatalf("err = %v, calls = %v, stdout = %q, want only the criterion handler", err, calls, stdout)
	}
}
