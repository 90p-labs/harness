package cli_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"go.opentelemetry.io/otel/trace/noop"

	"github.com/90p-labs/harness/internal/run/application/command/start"
	"github.com/90p-labs/harness/internal/run/application/command/transition"
	"github.com/90p-labs/harness/internal/run/application/command/waitpr"
	"github.com/90p-labs/harness/internal/run/application/query/report"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
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

type majorFinding struct{}

func (majorFinding) Read(context.Context, domain.Run) ([]shareddomain.ReviewFinding, error) {
	return []shareddomain.ReviewFinding{{ID: "F1", Severity: shareddomain.SeverityMajor, Verdict: shareddomain.FindingReal, Status: shareddomain.FindingOpen}}, nil
}

type passingGate struct{}

func (passingGate) Submit(context.Context, domain.Run) (domain.Submission, error) {
	return domain.Submission{}, nil
}
func (passingGate) Hash(domain.Run) (string, error) { return "", nil }

func (passingGate) Keep(domain.Run) error { return nil }

func (passingGate) StructureKept(domain.Run, string) (bool, error) { return false, nil }

type noApprovals struct{ domain.ApprovalRepository }

func (noApprovals) Save(context.Context, string, string, time.Time) error { return nil }

type silentHooks struct{}

func (silentHooks) Before(context.Context, domain.Run, domain.Command, domain.Phase) error {
	return nil
}
func (silentHooks) After(context.Context, domain.Run, domain.Run) {}

type noTrace struct{}

func (noTrace) Under(ctx context.Context, _ domain.Run) context.Context { return ctx }
func (noTrace) End(context.Context, domain.Run)                         {}

func TestTransitionCommand(t *testing.T) {
	cases := []struct {
		name      string
		phase     domain.Phase
		args      []string
		wantErr   error
		wantSaved int
		wantOut   string
		wantLine  bool
	}{
		{"approve by a model moves the run and asks for the handoff", domain.PhaseAwaitingApproval,
			[]string{"approve", "--by", "fable", "--reason", "the user approved all plans"}, nil, 1, "phase=working", true},
		{"plan moves the run to a phase that releases the turn", domain.PhasePlanning, []string{"plan"}, nil, 1, "phase=awaiting_approval", false},
		{"plan in the wrong phase is illegal", domain.PhaseAwaitingApproval, []string{"plan"}, domain.ErrIllegalTransition, 0, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &memoryRepository{stored: domain.Run{ID: submitRunID, Phase: tc.phase}}
			connect := func(context.Context) (cli.Handlers, func() error, error) {
				flow := transition.NewFlow(repo, silentHooks{}, passingGate{}, noApprovals{}, domain.Policy{ApprovalNeeded: true, MaxReviewRounds: 3, MaxLiveRounds: 3}, time.Now, noop.NewTracerProvider().Tracer("test"), noTrace{})
				return cli.Handlers{
					Transition: transition.NewHandler(repo, flow, nil, domain.Policy{}),
					Status:     status.NewHandler(repo),
				}, func() error { return nil }, nil
			}
			cmd := cli.NewCommand(connect)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs(tc.args)

			err := cmd.ExecuteContext(t.Context())

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if len(repo.saved) != tc.wantSaved || !strings.Contains(out.String(), tc.wantOut) {
				t.Errorf("saves = %d, output = %q, want %d and %q", len(repo.saved), out.String(), tc.wantSaved, tc.wantOut)
			}
			if got := strings.Contains(out.String(), "handoff: write"); got != tc.wantLine {
				t.Errorf("output = %q, handoff write line present = %v, want %v", out.String(), got, tc.wantLine)
			}
		})
	}
}

func TestReworkClass(t *testing.T) {
	t.Run("the class is stored with the transition", func(t *testing.T) {
		repo := &memoryRepository{stored: domain.Run{ID: submitRunID, Phase: domain.PhaseReviewing}}
		connect := func(context.Context) (cli.Handlers, func() error, error) {
			flow := transition.NewFlow(repo, silentHooks{}, passingGate{}, nil, domain.Policy{MaxReviewRounds: 3}, time.Now, noop.NewTracerProvider().Tracer("test"), noTrace{})
			return cli.Handlers{Transition: transition.NewHandler(repo, flow, majorFinding{}, domain.Policy{MaxReviewRounds: 3}), Status: status.NewHandler(repo)}, func() error { return nil }, nil
		}
		cmd := cli.NewCommand(connect)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetArgs([]string{"rework", "--findings", "F1 major a.go:1", "--class", string(domain.ReworkSpecGap)})

		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}

		if len(repo.saved) != 1 || repo.saved[0].Events()[0].Class != domain.ReworkSpecGap {
			t.Fatalf("saved = %+v, want class %s", repo.saved, domain.ReworkSpecGap)
		}
	})

	t.Run("an unknown class is refused before the transition", func(t *testing.T) {
		connect := func(context.Context) (cli.Handlers, func() error, error) {
			t.Fatal("the command connected with an unknown class")
			return cli.Handlers{}, nil, nil
		}
		cmd := cli.NewCommand(connect)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"rework", "--findings", "F1", "--class", "bogus"})

		err := cmd.ExecuteContext(t.Context())

		if err == nil || !strings.Contains(err.Error(), "bogus") {
			t.Fatalf("err = %v, want an unknown class error", err)
		}
	})
}

func TestApproveCommandFlags(t *testing.T) {
	for _, args := range [][]string{
		{"approve", "--by", "fable"},
		{"approve", "--reason", "the user approved all plans"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			connect := func(context.Context) (cli.Handlers, func() error, error) {
				t.Fatal("the command connected before the check of the flags")
				return cli.Handlers{}, nil, nil
			}
			cmd := cli.NewCommand(connect)
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(args)

			err := cmd.ExecuteContext(t.Context())

			if err == nil || !strings.Contains(err.Error(), `flags "by" and "reason" go together`) {
				t.Fatalf("err = %v, want the paired flags error", err)
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

func TestWaitCommand(t *testing.T) {
	const url = "https://github.com/90p-labs/harness/pull/107"
	refused := []struct {
		name    string
		args    []string
		wantErr error
		wantMsg string
	}{
		{"missing flag", []string{"wait"}, nil, `required flag "pr" not set`},
		{"not a pull request url", []string{"wait", "--pr", "https://example.com/1"}, shareddomain.ErrInvalidPRURL, "invalid pull request url"},
	}
	for _, tc := range refused {
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

	t.Run("a valid url is recorded and the status prints", func(t *testing.T) {
		repo := &memoryRepository{stored: domain.Run{ID: submitRunID, Phase: domain.PhaseShipping}}
		connect := func(context.Context) (cli.Handlers, func() error, error) {
			return cli.Handlers{
				WaitPR: waitpr.NewHandler(repo, noop.NewTracerProvider().Tracer("test")),
				Status: status.NewHandler(repo),
			}, func() error { return nil }, nil
		}
		cmd := cli.NewCommand(connect)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetArgs([]string{"wait", "--pr", url})

		if err := cmd.ExecuteContext(t.Context()); err != nil {
			t.Fatal(err)
		}

		if len(repo.saved) != 1 || repo.saved[0].PRURL != url {
			t.Fatalf("saved = %+v, want one run with PRURL %q", repo.saved, url)
		}
		if !strings.Contains(out.String(), "phase=shipping") || !strings.Contains(out.String(), "next: ") {
			t.Errorf("output = %q, want the status lines", out.String())
		}
	})
}

func TestShippingHintNamesWait(t *testing.T) {
	lines := cli.Lines(status.Status{ID: submitRunID, Phase: domain.PhaseShipping})

	if got := lines[len(lines)-1]; !strings.Contains(got, "harness run wait --pr") {
		t.Errorf("hint = %q, want it to name harness run wait --pr", got)
	}
}

func TestShippingHintNamesLearn(t *testing.T) {
	lines := cli.Lines(status.Status{ID: submitRunID, Phase: domain.PhaseShipping})

	got := lines[len(lines)-1]
	learnAt := strings.Index(got, "harness run learn")
	if learnAt < 0 || learnAt > strings.Index(got, "pull request") {
		t.Errorf("hint = %q, want it to name harness run learn before the pull request", got)
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

func TestHandoffLines(t *testing.T) {
	ready := status.Status{ID: "r1", Phase: domain.PhaseWorking, Step: 3, HandoffStep: 3}
	stale := ready
	stale.HandoffStep = 2
	fresh := status.Status{ID: "r1", Phase: domain.PhasePlanning}

	cases := []struct {
		name string
		in   status.Status
		want bool
	}{
		{"ready", ready, true},
		{"stale", stale, false},
		{"step zero", fresh, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Contains(strings.Join(cli.Lines(tc.in), "\n"), "handoff: ready (step 3)")
			if got != tc.want {
				t.Errorf("ready line present = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("write line", func(t *testing.T) {
		want := `handoff: write .harness/runs/r1/handoff.md with Phase: working, Step: 3 and the sections ## Next, ## Open findings, ## Unreviewed commits, ## Decisions, each with a "- " line, then continue`
		got := cli.HandoffWriteLine(ready)
		if got != want {
			t.Errorf("line = %q, want %q", got, want)
		}
		if strings.Contains(got, "end the turn") {
			t.Errorf("line = %q, must not tell the lead to end the turn", got)
		}
		if none := cli.HandoffWriteLine(status.Status{ID: "r1", Phase: domain.PhaseAwaitingApproval, Step: 3}); none != "" {
			t.Errorf("line = %q, want none for a phase that releases the turn", none)
		}
	})
}

func TestHandoffWriteLineNamesSections(t *testing.T) {
	s := status.Status{ID: "r1", Phase: domain.PhaseWorking, Step: 3}
	want := `handoff: write .harness/runs/r1/handoff.md with Phase: working, Step: 3 and the sections ## Next, ## Open findings, ## Unreviewed commits, ## Decisions, each with a "- " line, then continue`
	if got := cli.HandoffWriteLine(s); got != want {
		t.Errorf("line = %q, want %q", got, want)
	}
}

func TestAmendHelp(t *testing.T) {
	cmd := cli.NewCommand(nil)
	for _, sub := range cmd.Commands() {
		if sub.Name() != "amend" {
			continue
		}
		if sub.Short != "Amend the approved plan" {
			t.Errorf("amend help = %q, want %q", sub.Short, "Amend the approved plan")
		}
		if strings.Contains(sub.Short, "Reopen") {
			t.Errorf("amend help = %q, must not contain Reopen", sub.Short)
		}
		return
	}
	t.Fatal("amend command not found")
}

func TestTransitionDoesNotPrintModels(t *testing.T) {
	repo := &memoryRepository{stored: domain.Run{ID: submitRunID, Phase: domain.PhasePlanning}}
	connect := func(context.Context) (cli.Handlers, func() error, error) {
		flow := transition.NewFlow(repo, silentHooks{}, passingGate{}, noApprovals{}, domain.Policy{ApprovalNeeded: true, MaxReviewRounds: 3, MaxLiveRounds: 3}, time.Now, noop.NewTracerProvider().Tracer("test"), noTrace{})
		return cli.Handlers{
			Transition: transition.NewHandler(repo, flow, nil, domain.Policy{}),
			Status:     status.NewHandler(repo),
			Models:     []cli.HostModels{{Host: "claude", Roles: map[string][]string{"lead": {"cL"}}}},
		}, func() error { return nil }, nil
	}
	cmd := cli.NewCommand(connect)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"plan"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "models") {
		t.Errorf("a transition printed the models line:\n%s", out.String())
	}
}

func TestStatusPrintsModels(t *testing.T) {
	roles := func(prefix string) map[string][]string {
		return map[string][]string{
			"lead": {prefix + "L"}, "explorer": {prefix + "E"}, "worker": {prefix + "W1", prefix + "W2"},
			"reviewer": {prefix + "R1", prefix + "R2", prefix + "R3"}, "advisor": {prefix + "A1", prefix + "A2"},
		}
	}
	cases := map[string]struct {
		models []cli.HostModels
		want   []string
		absent bool
		never  string
	}{
		"one host": {
			models: []cli.HostModels{{Host: "claude", Roles: roles("c")}},
			want:   []string{"models: lead=cL explorer=cE worker=cW1,cW2 reviewer=cR1,cR2,cR3 advisor=cA1,cA2"},
		},
		"two hosts or no host": {
			models: []cli.HostModels{{Host: "claude", Roles: roles("c")}, {Host: "codex", Roles: roles("x")}},
			want: []string{
				"models (claude): lead=cL explorer=cE worker=cW1,cW2 reviewer=cR1,cR2,cR3 advisor=cA1,cA2",
				"models (codex): lead=xL explorer=xE worker=xW1,xW2 reviewer=xR1,xR2,xR3 advisor=xA1,xA2",
			},
		},
		"no host": {models: nil, want: nil, absent: true},
		"a host with no roles is skipped": {
			models: []cli.HostModels{{Host: "claude", Roles: roles("c")}, {Host: "codex"}},
			want:   []string{"models: lead=cL explorer=cE worker=cW1,cW2 reviewer=cR1,cR2,cR3 advisor=cA1,cA2"},
			never:  "codex",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &memoryRepository{stored: domain.Run{ID: submitRunID, Phase: domain.PhasePlanning}}
			connect := func(context.Context) (cli.Handlers, func() error, error) {
				return cli.Handlers{Status: status.NewHandler(repo), Models: tc.models}, func() error { return nil }, nil
			}
			cmd := cli.NewCommand(connect)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{"status"})

			if err := cmd.ExecuteContext(t.Context()); err != nil {
				t.Fatal(err)
			}

			for _, line := range tc.want {
				if !strings.Contains(out.String(), line+"\n") {
					t.Errorf("output lacks %q:\n%s", line, out.String())
				}
			}
			if tc.absent && strings.Contains(out.String(), "models") {
				t.Errorf("output holds a models line:\n%s", out.String())
			}
			if tc.never != "" && strings.Contains(out.String(), tc.never) {
				t.Errorf("output holds %q:\n%s", tc.never, out.String())
			}
		})
	}
}

func TestStartPrintsModels(t *testing.T) {
	now := time.Date(2026, 11, 10, 8, 20, 0, 0, time.UTC)
	store := &reportStore{}
	models := []cli.HostModels{{Host: "claude", Roles: map[string][]string{
		"lead": {"L"}, "explorer": {"E"}, "worker": {"W1", "W2"}, "reviewer": {"R"}, "advisor": {"A"},
	}}}
	connect := func(context.Context) (cli.Handlers, func() error, error) {
		flow := transition.NewFlow(store, silentHooks{}, passingGate{}, nil, domain.Policy{}, func() time.Time { return now }, noop.NewTracerProvider().Tracer("test"), noTrace{})
		return cli.Handlers{
			Key:    reportKey,
			Start:  start.NewHandler(store, flow, func() time.Time { return now }),
			Status: status.NewHandler(store),
			Report: report.NewHandler(store, store, func() time.Time { return now },
				func(id string) string { return "trace-of-" + id }, noop.NewTracerProvider().Tracer("test")),
			Models: models,
		}, func() error { return nil }, nil
	}
	cmd := cli.NewCommand(connect)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"start", "add login"})

	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}

	const line = "models: lead=L explorer=E worker=W1,W2 reviewer=R advisor=A\n"
	lines := strings.SplitAfter(out.String(), "\n")
	if len(lines) < 2 || lines[1] != line {
		t.Fatalf("output:\n%s\nwant the second line %q", out.String(), line)
	}
}

const gitignoreLine = "gitignore: added .harness/runs/\n"

func startWithIgnore(store *reportStore, ignore func() (bool, error), args ...string) (string, error) {
	return startWithStatus(store, store, ignore, args...)
}

// startWithStatus runs start with the status query on statusStore, so a test can make the query fail after the start.
func startWithStatus(store, statusStore *reportStore, ignore func() (bool, error), args ...string) (string, error) {
	now := time.Date(2026, 11, 10, 8, 20, 0, 0, time.UTC)
	connect := func(context.Context) (cli.Handlers, func() error, error) {
		flow := transition.NewFlow(store, silentHooks{}, passingGate{}, nil, domain.Policy{}, func() time.Time { return now }, noop.NewTracerProvider().Tracer("test"), noTrace{})
		return cli.Handlers{
			Key:    reportKey,
			Start:  start.NewHandler(store, flow, func() time.Time { return now }),
			Status: status.NewHandler(statusStore),
			Report: report.NewHandler(store, store, func() time.Time { return now },
				func(id string) string { return "trace-of-" + id }, noop.NewTracerProvider().Tracer("test")),
			Models:     []cli.HostModels{{Host: "claude", Roles: map[string][]string{"lead": {"L"}, "explorer": {"E"}, "worker": {"W"}, "reviewer": {"R"}, "advisor": {"A"}}}},
			IgnoreRuns: ignore,
		}, func() error { return nil }, nil
	}
	cmd := cli.NewCommand(connect)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(append([]string{"start"}, args...))
	err := cmd.ExecuteContext(context.Background())
	return out.String(), err
}

func TestStartPrintsTheGitignoreLine(t *testing.T) {
	out, err := startWithIgnore(&reportStore{}, func() (bool, error) { return true, nil }, "add login")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.SplitAfter(out, "\n")
	if len(lines) < 3 || !strings.HasPrefix(lines[1], "models: ") || lines[2] != gitignoreLine {
		t.Fatalf("output:\n%s\nwant the models line, then %q", out, gitignoreLine)
	}

	out, err = startWithIgnore(&reportStore{}, func() (bool, error) { return false, nil }, "add login")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "gitignore:") {
		t.Fatalf("a kept file printed a gitignore line:\n%s", out)
	}
}

func TestStartStopsOnAGitignoreError(t *testing.T) {
	store := &reportStore{}
	boom := errors.New("write .gitignore: denied")
	out, err := startWithIgnore(store, func() (bool, error) { return false, boom }, "add login")
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if store.run != nil {
		t.Fatalf("a run exists after the failed gitignore step: %+v", store.run)
	}
	if strings.Contains(out, "gitignore:") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestStartReportsTheGitignoreOnAFailedStart(t *testing.T) {
	store := &reportStore{}
	if _, err := startWithIgnore(store, nil, "first"); err != nil {
		t.Fatal(err)
	}
	out, err := startWithIgnore(store, func() (bool, error) { return true, nil }, "second")
	if err == nil {
		t.Fatal("the second start with an active run succeeded")
	}
	if !strings.HasPrefix(out, gitignoreLine) {
		t.Fatalf("output %q, want it to start with %q", out, gitignoreLine)
	}

	out, err = startWithStatus(&reportStore{}, &reportStore{}, func() (bool, error) { return true, nil }, "status fails")
	if !errors.Is(err, domain.ErrRunNotFound) {
		t.Fatalf("err = %v, want the failed status query", err)
	}
	if !strings.HasPrefix(out, gitignoreLine) || strings.Count(out, "gitignore:") != 1 {
		t.Fatalf("output %q, want it to start with %q once after the failed status query", out, gitignoreLine)
	}
}
