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
	"github.com/90p-labs/harness/internal/run/application/query/report"
	"github.com/90p-labs/harness/internal/run/application/query/status"
	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/ui/cli"
)

var reportKey = domain.RunKey{Repo: "github.com/90p-labs/harness", Worktree: "/work/harness"}

type reportStore struct {
	run    *domain.Run
	rows   []domain.Transitioned
	reps   []domain.ClassCount
	repErr error
}

func (s *reportStore) Active(_ context.Context, k domain.RunKey) (domain.Run, error) {
	if s.run == nil || s.run.Key != k {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return *s.run, nil
}

func (s *reportStore) Get(_ context.Context, id string) (domain.Run, error) {
	if s.run == nil || s.run.ID != id {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return *s.run, nil
}

func (*reportStore) ListByPhase(context.Context, string, []domain.Phase) ([]domain.Run, error) {
	return nil, nil
}

func (s *reportStore) Save(_ context.Context, run domain.Run) (domain.Run, error) {
	s.run = &run
	return run, nil
}

func (s *reportStore) Transitions(context.Context, string) ([]domain.Transitioned, error) {
	return s.rows, nil
}

func (s *reportStore) RepeatClasses(context.Context, string) ([]domain.ClassCount, error) {
	return s.reps, s.repErr
}

func runWith(t *testing.T, store *reportStore, now time.Time, args ...string) string {
	t.Helper()
	out, _ := runWithStreams(t, store, now, args...)
	return out
}

func runWithStreams(t *testing.T, store *reportStore, now time.Time, args ...string) (string, string) {
	t.Helper()
	connect := func(context.Context) (cli.Handlers, func() error, error) {
		flow := transition.NewFlow(store, silentHooks{}, passingGate{}, nil, domain.Policy{}, func() time.Time { return now }, noop.NewTracerProvider().Tracer("test"), noTrace{})
		return cli.Handlers{
			Key:        reportKey,
			Start:      start.NewHandler(store, flow, func() time.Time { return now }),
			Transition: transition.NewHandler(store, flow, majorFinding{}, domain.Policy{}),
			Status:     status.NewHandler(store),
			Report: report.NewHandler(store, store, func() time.Time { return now },
				func(id string) string { return "trace-of-" + id }, noop.NewTracerProvider().Tracer("test")),
		}, func() error { return nil }, nil
	}
	cmd := cli.NewCommand(connect)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.ExecuteContext(t.Context()); err != nil {
		t.Fatal(err)
	}
	return out.String(), errOut.String()
}

func TestReportCommand(t *testing.T) {
	started := time.Date(2026, 11, 10, 8, 20, 0, 0, time.UTC)
	store := &reportStore{
		run: &domain.Run{ID: submitRunID, Key: reportKey, Phase: domain.PhaseReviewing, StartedAt: started, StopBlocksTotal: 2},
		rows: []domain.Transitioned{
			{Event: domain.EventStart, To: domain.PhasePlanning, At: started},
			{Event: domain.EventSubmitPlan, From: domain.PhasePlanning, To: domain.PhaseWorking, At: started.Add(18*time.Minute + 12*time.Second)},
			{Event: domain.EventSubmitForReview, From: domain.PhaseWorking, To: domain.PhaseReviewing, At: started.Add(60 * time.Minute)},
			{Event: domain.EventRequestRework, From: domain.PhaseReviewing, To: domain.PhaseWorking, Class: domain.ReworkImplementation, At: started.Add(61 * time.Minute)},
			{Event: domain.EventSubmitForReview, From: domain.PhaseWorking, To: domain.PhaseReviewing, At: started.Add(70 * time.Minute)},
		},
	}
	want := strings.Join([]string{
		"run " + submitRunID + " phase=reviewing",
		"rework rounds: 1",
		"amends: 0",
		"stop blocks: 2",
		"rework classes: implementation 1",
		"minutes: planning 18.2, working 50.8, reviewing 11.0",
		"trace: trace-of-" + submitRunID,
		"",
	}, "\n")

	for name, args := range map[string][]string{
		"no argument reads the active run": {"report"},
		"a run id reads that run":          {"report", submitRunID},
	} {
		t.Run(name, func(t *testing.T) {
			got := runWith(t, store, started.Add(80*time.Minute), args...)

			if got != want {
				t.Fatalf("output:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestReportCommandWithNoRework(t *testing.T) {
	started := time.Date(2026, 11, 10, 8, 20, 0, 0, time.UTC)
	store := &reportStore{
		run:  &domain.Run{ID: submitRunID, Key: reportKey, Phase: domain.PhasePlanning, StartedAt: started},
		rows: []domain.Transitioned{{Event: domain.EventStart, To: domain.PhasePlanning, At: started}},
	}

	got := runWith(t, store, started.Add(3*time.Minute), "report")

	if !strings.Contains(got, "rework classes: none\n") || !strings.Contains(got, "minutes: planning 3.0\n") {
		t.Fatalf("output:\n%s", got)
	}
}

func TestRepeatClassesAtStart(t *testing.T) {
	now := time.Date(2026, 11, 10, 8, 20, 0, 0, time.UTC)

	t.Run("the start prints the repeat classes", func(t *testing.T) {
		store := &reportStore{reps: []domain.ClassCount{
			{Class: domain.ReworkImplementation, Count: 32}, {Class: domain.ReworkMinor, Count: 7},
			{Class: domain.ReworkSpecGap, Count: 6}, {Class: domain.ReworkGateCheckable, Count: 5},
		}}

		got := runWith(t, store, now, "start", "add login")

		const line = "repeat rework classes: implementation 32, minor 7, spec-gap 6, gate-checkable 5\n"
		if !strings.HasSuffix(got, "\n"+line) {
			t.Fatalf("output:\n%s\nwant the last line %q", got, line)
		}
	})

	t.Run("a failing repeat query warns and the start succeeds", func(t *testing.T) {
		store := &reportStore{repErr: errors.New("db down")}

		out, errOut := runWithStreams(t, store, now, "start", "add login")

		if store.run == nil || strings.Contains(out, "repeat rework classes") || !strings.Contains(errOut, "db down") {
			t.Fatalf("run = %v, stdout:\n%s\nstderr:\n%s", store.run, out, errOut)
		}
	})

	t.Run("the start prints no line without a repeat class", func(t *testing.T) {
		got := runWith(t, &reportStore{}, now, "start", "add login")

		if strings.Contains(got, "repeat rework classes") {
			t.Fatalf("output:\n%s", got)
		}
	})
}
