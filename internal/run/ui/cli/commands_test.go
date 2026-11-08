package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/jorge07/harness/internal/run/application/command/transition"
	"github.com/jorge07/harness/internal/run/application/query/status"
	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/run/ui/cli"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	submitRunID = "20261108-011805-add-the-login-page"
	planHint    = ".harness/runs/" + submitRunID + "/plan.md"
)

type memoryRepository struct {
	stored domain.Run
	saved  []domain.Run
}

func (m *memoryRepository) Active(context.Context, domain.RunKey) (domain.Run, error) {
	return m.stored, nil
}

func (m *memoryRepository) Get(context.Context, string) (domain.Run, error) {
	if len(m.saved) > 0 {
		return m.saved[len(m.saved)-1], nil
	}
	return m.stored, nil
}

func (m *memoryRepository) ListByPhase(context.Context, string, []domain.Phase) ([]domain.Run, error) {
	return nil, nil
}

func (m *memoryRepository) Save(_ context.Context, run domain.Run) (domain.Run, error) {
	m.saved = append(m.saved, run)
	return run, nil
}

type silentHooks struct{}

func (silentHooks) Before(context.Context, domain.Run, domain.Command) error { return nil }
func (silentHooks) After(context.Context, domain.Run, domain.Run)            {}

func TestTransitionCommand(t *testing.T) {
	cases := []struct {
		name      string
		phase     domain.Phase
		args      string
		wantErr   error
		wantSaved int
		wantOut   string
	}{
		{"approve moves the run", domain.PhaseAwaitingApproval, "approve", nil, 1, "phase=working"},
		{"plan in the wrong phase is illegal", domain.PhaseAwaitingApproval, "plan", domain.ErrIllegalTransition, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memoryRepository{stored: domain.Run{ID: submitRunID, Phase: tc.phase}}
			connect := func(context.Context) (cli.Handlers, func() error, error) {
				flow := transition.NewFlow(repo, silentHooks{}, nil, domain.Policy{ApprovalNeeded: true, MaxReviewRounds: 3, MaxLiveRounds: 3}, time.Now, noop.NewTracerProvider().Tracer("test"))
				return cli.Handlers{
					Transition: transition.NewHandler(repo, flow),
					Status:     status.NewHandler(repo),
				}, func() error { return nil }, nil
			}
			cmd := cli.NewCommand(connect)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{tc.args})

			err := cmd.ExecuteContext(t.Context())

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if len(repo.saved) != tc.wantSaved || !strings.Contains(out.String(), tc.wantOut) {
				t.Errorf("saves = %d, output = %q, want %d and %q", len(repo.saved), out.String(), tc.wantSaved, tc.wantOut)
			}
		})
	}
}

func TestDoneChecksThePRFlag(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr error
		wantMsg string
	}{
		{"missing flag", []string{"done"}, nil, `required flag "pr" not set`},
		{"not a pull request url", []string{"done", "--pr", "https://example.com/1"}, shareddomain.ErrInvalidPRURL, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			connect := func(context.Context) (cli.Handlers, func() error, error) {
				t.Fatal("the command connected before the check of the flag")
				return cli.Handlers{}, nil, nil
			}
			cmd := cli.NewCommand(connect)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(t.Context())

			if err == nil || (tc.wantErr != nil && !errors.Is(err, tc.wantErr)) || !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err = %v, want %v and %q", err, tc.wantErr, tc.wantMsg)
			}
		})
	}
}

func TestPlanningHintNamesThePlanFile(t *testing.T) {
	lines := cli.Lines(status.Status{ID: submitRunID, Phase: domain.PhasePlanning})

	if got := lines[len(lines)-1]; !strings.Contains(got, planHint) {
		t.Errorf("hint = %q, want it to name %q", got, planHint)
	}
}

func TestStatusLines(t *testing.T) {
	working := status.Status{
		ID: "20261108-011805-add-the-login-page", Phase: domain.PhaseWorking,
		ReviewRound: 1, LiveRound: 0,
	}
	withReason := working
	withReason.Reason = "the tests miss the empty case"

	cases := []struct {
		name string
		in   status.Status
		want []string
	}{
		{"no reason", working, []string{
			"run 20261108-011805-add-the-login-page phase=working review_round=1 live_round=0",
			"next: implement test first, run harness verify, then harness run review",
		}},
		{"reason", withReason, []string{
			"run 20261108-011805-add-the-login-page phase=working review_round=1 live_round=0",
			"reason: the tests miss the empty case",
			"next: implement test first, run harness verify, then harness run review",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cli.Lines(tc.in)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("Lines = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStatusLinesHintForEachPhase(t *testing.T) {
	phases := []domain.Phase{
		domain.PhasePlanning, domain.PhaseAwaitingApproval, domain.PhaseWorking, domain.PhaseReviewing,
		domain.PhaseShipping, domain.PhaseReleasing, domain.PhaseObserving, domain.PhaseBlocked,
		domain.PhaseDone, domain.PhaseAbandoned,
	}
	for _, phase := range phases {
		t.Run(string(phase), func(t *testing.T) {
			lines := cli.Lines(status.Status{ID: "r1", Phase: phase})
			last := lines[len(lines)-1]
			if !strings.HasPrefix(last, "next: ") || strings.TrimPrefix(last, "next: ") == "" {
				t.Errorf("last line = %q, want a next hint", last)
			}
		})
	}
}
