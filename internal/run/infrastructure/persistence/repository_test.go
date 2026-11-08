package persistence_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/run/infrastructure/persistence"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/dbal"
)

var (
	clock = time.Date(2026, 11, 8, 12, 30, 0, 0, time.FixedZone("CET", 3600))
	key   = domain.RunKey{Repo: "github.com/jorge07/harness", Worktree: "/repo"}
)

type dialectCase struct {
	name string
	dsn  func(t *testing.T) string
}

var dialectCases = []dialectCase{
	{name: "sqlite", dsn: func(t *testing.T) string { return "sqlite:" + filepath.Join(t.TempDir(), "state.db") }},
}

type store struct {
	db   *dbal.DB
	repo persistence.SQLRunRepository
}

func openStore(t *testing.T, c dialectCase) store {
	t.Helper()
	ctx := context.Background()
	db, err := dbal.Open(ctx, c.dsn(t), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(ctx, persistence.Module, persistence.Migrations); err != nil {
		t.Fatal(err)
	}
	return store{db: db, repo: persistence.NewSQLRunRepository(db, func() time.Time { return clock })}
}

func newRun(t *testing.T, id string, events ...domain.Event) domain.Run {
	t.Helper()
	run := domain.Run{ID: id, Key: key, Prompt: "add the run command\nwith a second line", Track: shareddomain.TrackFull}
	for _, event := range events {
		next, err := run.Apply(domain.Command{Event: event, Reason: string(event), At: clock}, domain.Policy{})
		if err != nil {
			t.Fatal(err)
		}
		run = next
	}
	return run
}

func save(t *testing.T, s store, run domain.Run) domain.Run {
	t.Helper()
	saved, err := s.repo.Save(context.Background(), run)
	if err != nil {
		t.Fatalf("save %s: %v", run.ID, err)
	}
	return saved
}

func get(t *testing.T, s store, id string) domain.Run {
	t.Helper()
	run, err := s.repo.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("get %s: %v", id, err)
	}
	return run
}

func transitionCount(t *testing.T, s store, id string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow("SELECT count(*) FROM run_transitions WHERE run_id = ?", id).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestRunRepositoryContract(t *testing.T) {
	ctx := context.Background()
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("SaveAndGet", func(t *testing.T) {
				s := openStore(t, c)
				saved := save(t, s, newRun(t, "r1", domain.EventStart))

				if saved.Version != 1 || saved.Events() != nil {
					t.Fatalf("saved = %+v, events = %+v", saved, saved.Events())
				}
				if got := get(t, s, "r1"); !reflect.DeepEqual(got, saved) {
					t.Fatalf("got %+v, want %+v", got, saved)
				}
				var created, at time.Time
				if err := s.db.QueryRow("SELECT created_at FROM runs WHERE id = ?", "r1").Scan(&created); err != nil {
					t.Fatal(err)
				}
				if err := s.db.QueryRow("SELECT at FROM run_transitions WHERE run_id = ?", "r1").Scan(&at); err != nil {
					t.Fatal(err)
				}
				for _, stored := range []time.Time{created, at} {
					if _, offset := stored.Zone(); !stored.Equal(clock) || offset != 0 {
						t.Fatalf("stored time = %v, want %v in UTC", stored, clock.UTC())
					}
				}
				if _, err := s.repo.Get(ctx, "missing"); !errors.Is(err, domain.ErrRunNotFound) {
					t.Fatalf("get missing: %v", err)
				}
			})

			t.Run("UpdateRaisesTrack", func(t *testing.T) {
				s := openStore(t, c)
				started := newRun(t, "r1", domain.EventStart)
				started.Track = ""
				saved := save(t, s, started)
				next, err := saved.Apply(domain.Command{Event: domain.EventSubmitPlan, Track: shareddomain.TrackFull, At: clock}, domain.Policy{})
				if err != nil {
					t.Fatal(err)
				}

				save(t, s, next)

				if got := get(t, s, "r1").Track; got != shareddomain.TrackFull {
					t.Fatalf("track = %q, want %q", got, shareddomain.TrackFull)
				}
			})

			t.Run("ActiveByKey", func(t *testing.T) {
				s := openStore(t, c)
				for id, phase := range map[string]domain.Phase{"done": domain.PhaseDone, "abandoned": domain.PhaseAbandoned} {
					closed := newRun(t, id)
					closed.Phase = phase
					save(t, s, closed)
				}

				if _, err := s.repo.Active(ctx, key); !errors.Is(err, domain.ErrRunNotFound) {
					t.Fatalf("active with only closed runs: %v", err)
				}
				open := save(t, s, newRun(t, "new", domain.EventStart))
				got, err := s.repo.Active(ctx, key)
				if err != nil || !reflect.DeepEqual(got, open) {
					t.Fatalf("active = %+v, %v, want %+v", got, err, open)
				}
			})

			t.Run("StaleVersion", func(t *testing.T) {
				s := openStore(t, c)
				saved := save(t, s, newRun(t, "r1", domain.EventStart))
				next, err := saved.Apply(domain.Command{Event: domain.EventSubmitPlan, At: clock}, domain.Policy{})
				if err != nil {
					t.Fatal(err)
				}
				current := save(t, s, next)

				if _, err := s.repo.Save(ctx, next); !errors.Is(err, domain.ErrStaleRun) {
					t.Fatalf("stale save: %v", err)
				}
				if got := get(t, s, "r1"); !reflect.DeepEqual(got, current) {
					t.Fatalf("got %+v, want %+v", got, current)
				}
			})

			t.Run("OneActivePerKey", func(t *testing.T) {
				s := openStore(t, c)
				save(t, s, newRun(t, "r1", domain.EventStart))

				if _, err := s.repo.Save(ctx, newRun(t, "r2", domain.EventStart)); !errors.Is(err, domain.ErrActiveRunExists) {
					t.Fatalf("second open run: %v", err)
				}
				done := newRun(t, "r3")
				done.Phase = domain.PhaseDone
				save(t, s, done)
			})

			t.Run("DuplicateID", func(t *testing.T) {
				s := openStore(t, c)
				done := newRun(t, "r1")
				done.Phase = domain.PhaseDone
				save(t, s, done)

				clash := newRun(t, "r1", domain.EventStart)
				clash.Key = domain.RunKey{Repo: key.Repo, Worktree: "/other"}
				_, err := s.repo.Save(ctx, clash)
				if err == nil || errors.Is(err, domain.ErrActiveRunExists) {
					t.Fatalf("duplicate id: %v, want an error other than ErrActiveRunExists", err)
				}
			})

			t.Run("TransitionsAtomic", func(t *testing.T) {
				s := openStore(t, c)
				orphan := newRun(t, "ghost", domain.EventStart)
				orphan.ID = "r1"

				if _, err := s.repo.Save(ctx, orphan); err == nil {
					t.Fatal("insert with an orphan transition saved")
				}
				if _, err := s.repo.Get(ctx, "r1"); !errors.Is(err, domain.ErrRunNotFound) {
					t.Fatalf("run row after a failed insert: %v", err)
				}

				saved := save(t, s, newRun(t, "r2", domain.EventStart))
				ghost := saved
				ghost.ID = "ghost"
				update, err := ghost.Apply(domain.Command{Event: domain.EventSubmitPlan, At: clock}, domain.Policy{})
				if err != nil {
					t.Fatal(err)
				}
				update.ID = "r2"

				if _, err := s.repo.Save(ctx, update); err == nil {
					t.Fatal("update with an orphan transition saved")
				}
				if got := get(t, s, "r2"); !reflect.DeepEqual(got, saved) {
					t.Fatalf("got %+v, want %+v", got, saved)
				}
				if n := transitionCount(t, s, "r1") + transitionCount(t, s, "ghost"); n != 0 {
					t.Fatalf("orphan transitions = %d, want 0", n)
				}
				if n := transitionCount(t, s, "r2"); n != 1 {
					t.Fatalf("r2 transitions = %d, want 1", n)
				}
			})
		})
	}
}
