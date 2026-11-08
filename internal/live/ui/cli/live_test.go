package cli_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/jorge07/harness/internal/live/application/command/check"
	"github.com/jorge07/harness/internal/live/domain"
	"github.com/jorge07/harness/internal/live/ui/cli"
)

var errGitHub = errors.New("gh is down")

func runCheck(t *testing.T, results []check.Result, passErr error) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := cli.NewCommand(func(context.Context) ([]check.Result, error) { return results, passErr })
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"check"})
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestLiveCheckCommand(t *testing.T) {
	healthy := check.Result{RunID: "r1", Phase: domain.PhaseObserving, Outcome: domain.OutcomeHealthy}
	pending := check.Result{RunID: "r3", Phase: domain.PhaseReleasing, Outcome: domain.OutcomePending}
	failed := check.Result{RunID: "r2", Phase: domain.PhaseReleasing, Outcome: domain.OutcomeError, Err: errGitHub}

	t.Run("a full pass prints one line for each run and exits 0", func(t *testing.T) {
		out, err := runCheck(t, []check.Result{healthy, pending}, nil)

		if err != nil {
			t.Fatalf("err = %v, want nil", err)
		}
		if want := "r1 observing healthy\nr3 releasing pending\n"; out != want {
			t.Fatalf("output = %q, want %q", out, want)
		}
	})

	t.Run("a failed run prints its error and the pass error is returned", func(t *testing.T) {
		out, err := runCheck(t, []check.Result{healthy, failed}, errGitHub)

		if !errors.Is(err, errGitHub) {
			t.Fatalf("err = %v, want the pass error", err)
		}
		if want := "r1 observing healthy\nr2 releasing error: gh is down\n"; out != want {
			t.Fatalf("output = %q, want %q", out, want)
		}
	})
}
