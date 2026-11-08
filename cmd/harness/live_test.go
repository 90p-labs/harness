package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.opentelemetry.io/otel"

	livedomain "github.com/jorge07/harness/internal/live/domain"
	runmodule "github.com/jorge07/harness/internal/run"
	"github.com/jorge07/harness/internal/run/application/query/status"
	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/run/infrastructure/persistence"
	runcli "github.com/jorge07/harness/internal/run/ui/cli"
	"github.com/jorge07/harness/internal/shared/application/eventbus"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/config"
)

const (
	liveRepo      = "o/r"
	otherRepo     = "o/other"
	livePR        = "https://github.com/o/r/pull/7"
	liveStateDSN  = "sqlite:state.db"
	pingSignal    = "ping calls"
	pingSource    = "prometheus"
	pingQuery     = "ping:calls:rate5m"
	pingBreachMax = 100
)

func seedRun(t *testing.T, repo domain.RunRepository, id, runRepo string, phase domain.Phase) {
	t.Helper()
	_, err := repo.Save(context.Background(), domain.Run{
		ID: id, Key: domain.RunKey{Repo: runRepo, Worktree: t.TempDir()}, Phase: phase, PRURL: livePR,
	})
	if err != nil {
		t.Fatalf("seed %s: %v", id, err)
	}
}

func liveState(t *testing.T) (runcli.Handlers, domain.RunRepository) {
	t.Helper()
	cfg := config.Config{State: config.State{DSN: liveStateDSN}}
	db, err := openState(context.Background(), cfg, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	bus := eventbus.NewBus(nil, otel.GetTracerProvider(), otel.GetMeterProvider())
	handlers := runmodule.NewModule(db, bus, planGate{}, cfg.Run, domain.RunKey{Repo: liveRepo}, time.Now)
	return handlers, persistence.NewSQLRunRepository(db, time.Now)
}

func TestLiveRunsAdapter(t *testing.T) {
	ctx := context.Background()
	handlers, repo := liveState(t)
	for id, spec := range map[string]struct {
		repo  string
		phase domain.Phase
	}{
		"releasing":  {liveRepo, domain.PhaseReleasing},
		"observing":  {liveRepo, domain.PhaseObserving},
		"working":    {liveRepo, domain.PhaseWorking},
		"elsewhere":  {otherRepo, domain.PhaseReleasing},
		"closed-one": {liveRepo, domain.PhaseDone},
	} {
		seedRun(t, repo, id, spec.repo, spec.phase)
	}
	runs := liveRuns{handlers: handlers}

	live, err := runs.Live(ctx, liveRepo)

	if err != nil {
		t.Fatalf("live: %v", err)
	}
	phases := map[string]livedomain.Phase{}
	for _, run := range live {
		phases[run.ID] = run.Phase
	}
	want := map[string]livedomain.Phase{"releasing": livedomain.PhaseReleasing, "observing": livedomain.PhaseObserving}
	if len(phases) != len(want) || phases["releasing"] != want["releasing"] || phases["observing"] != want["observing"] {
		t.Fatalf("live runs = %v, want %v", phases, want)
	}

	var releasing livedomain.LiveRun
	for _, run := range live {
		if run.ID == "releasing" {
			releasing = run
		}
	}
	if releasing.PRURL != livePR || releasing.Version != 1 || releasing.Worktree == "" {
		t.Fatalf("releasing run = %+v", releasing)
	}

	t.Run("a stale version is the live ErrStaleRun", func(t *testing.T) {
		stale := releasing
		stale.Version = 9

		err := runs.Apply(ctx, stale, livedomain.EventDeploy, "")

		if !errors.Is(err, livedomain.ErrStaleRun) {
			t.Fatalf("err = %v, want live ErrStaleRun", err)
		}
	})

	t.Run("deploy moves a releasing run to observing", func(t *testing.T) {
		if err := runs.Apply(ctx, releasing, livedomain.EventDeploy, ""); err != nil {
			t.Fatalf("apply: %v", err)
		}

		got, err := handlers.Status.Handle(ctx, status.Query{RunID: "releasing"})

		if err != nil || got.Phase != domain.PhaseObserving {
			t.Fatalf("status = %+v, err = %v, want observing", got, err)
		}
	})

	t.Run("signals converts a plan table row", func(t *testing.T) {
		run := livedomain.LiveRun{ID: "observing", Worktree: t.TempDir()}
		writeRunPlan(t, run.Worktree, run.ID, validPlan(t))

		got, err := runs.Signals(ctx, run)

		want := livedomain.Threshold{
			Signal: pingSignal, Source: pingSource, Query: pingQuery,
			Breach: shareddomain.Breach{Op: shareddomain.OperatorGreater, Value: pingBreachMax},
			Window: 24 * time.Hour, Panel: pingSignal,
		}
		if err != nil || len(got) != 1 || got[0] != want {
			t.Fatalf("signals = %+v, err = %v, want [%+v]", got, err, want)
		}
	})
}

func TestLiveCheckCommandRegistered(t *testing.T) {
	clearHarnessEnv(t)
	initRepo(t)

	code, stdout, stderr := execute(t, "live", "check")

	if code != exitOK || stdout != "" || stderr != "" {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

type recordingDispatcher struct{ got shareddomain.Envelope }

func (d *recordingDispatcher) Dispatch(_ context.Context, envelope shareddomain.Envelope) (shareddomain.DispatchResult, error) {
	d.got = envelope
	return shareddomain.DispatchResult{}, nil
}

func TestActiveRunRef(t *testing.T) {
	ctx := context.Background()
	key := domain.RunKey{Repo: liveRepo, Worktree: t.TempDir()}
	env := environment{key: key, commonDir: t.TempDir(), cfg: config.Config{State: config.State{DSN: liveStateDSN}}}

	if _, err := activeRunRef(env)(ctx); !errors.Is(err, domain.ErrRunNotFound) {
		t.Fatalf("no run: err = %v, want ErrRunNotFound", err)
	}

	db, err := openState(ctx, env.cfg, env.commonDir)
	if err != nil {
		t.Fatal(err)
	}
	_, err = persistence.NewSQLRunRepository(db, time.Now).Save(ctx, domain.Run{ID: "r1", Key: key, Phase: domain.PhaseWorking})
	if closeErr := db.Close(); err != nil || closeErr != nil {
		t.Fatalf("seed: %v, close: %v", err, closeErr)
	}
	ref, err := activeRunRef(env)(ctx)
	if err != nil || ref != (shareddomain.RunRef{ID: "r1", Phase: string(domain.PhaseWorking)}) {
		t.Fatalf("active run: ref = %+v, err = %v", ref, err)
	}

	env.cfg.State.DSN = "unknown:state.db"
	if _, err := activeRunRef(env)(ctx); err == nil {
		t.Fatal("bad state: want an error")
	}
}

func TestRunDispatcherFillsTheRun(t *testing.T) {
	errState := errors.New("state does not open")
	working := shareddomain.RunRef{ID: "r1", Phase: string(domain.PhaseWorking)}
	cases := []struct {
		name      string
		activeRun shareddomain.RunRef
		activeErr error
		want      shareddomain.RunRef
	}{
		{name: "the active run fills the envelope", activeRun: working, want: working},
		{name: "no active run leaves the run empty", activeRun: working, activeErr: domain.ErrRunNotFound},
		{name: "a state that does not open leaves the run empty", activeRun: working, activeErr: errState},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next := &recordingDispatcher{}
			active := func(context.Context) (shareddomain.RunRef, error) { return tc.activeRun, tc.activeErr }

			_, err := runDispatcher{next: next, active: active}.Dispatch(context.Background(), shareddomain.Envelope{Event: shareddomain.EventSessionStarted})

			if err != nil {
				t.Fatalf("dispatch: %v", err)
			}
			if next.got.Run != tc.want || next.got.Event != shareddomain.EventSessionStarted {
				t.Fatalf("envelope = %+v, want run %+v", next.got, tc.want)
			}
		})
	}
}
