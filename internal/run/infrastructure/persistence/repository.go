package persistence

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jorge07/harness/internal/run/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/dbal"
)

const (
	Module        = "run"
	migrationsDir = "migrations"

	runColumns       = "id, repo, worktree, prompt, track, phase, review_round, live_round, stop_blocks, blocked_from, reason, version"
	selectRun        = "SELECT " + runColumns + " FROM runs"
	selectRunByID    = selectRun + " WHERE id = ?"
	selectActiveRun  = selectRun + " WHERE repo = ? AND worktree = ? AND phase NOT IN (?, ?)"
	insertRun        = "INSERT INTO runs (" + runColumns + ", created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	updateRun        = "UPDATE runs SET track = ?, phase = ?, review_round = ?, live_round = ?, stop_blocks = ?, blocked_from = ?, reason = ?, version = ?, updated_at = ? WHERE id = ? AND version = ?"
	insertTransition = "INSERT INTO run_transitions (run_id, at, from_phase, event, to_phase, reason) VALUES (?, ?, ?, ?, ?, ?)"
)

//go:embed migrations
var embedded embed.FS

var Migrations = func() fs.FS {
	sub, err := fs.Sub(embedded, migrationsDir)
	if err != nil {
		panic(err)
	}
	return sub
}()

type SQLRunRepository struct {
	db  *dbal.DB
	now func() time.Time
}

func NewSQLRunRepository(db *dbal.DB, now func() time.Time) SQLRunRepository {
	return SQLRunRepository{db: db, now: now}
}

func (r SQLRunRepository) Active(ctx context.Context, key domain.RunKey) (domain.Run, error) {
	return r.one(ctx, selectActiveRun, key.Repo, key.Worktree, domain.PhaseDone, domain.PhaseAbandoned)
}

func (r SQLRunRepository) Get(ctx context.Context, id string) (domain.Run, error) {
	return r.one(ctx, selectRunByID, id)
}

func (r SQLRunRepository) Save(ctx context.Context, run domain.Run) (domain.Run, error) {
	now := r.now().UTC()
	err := r.db.InTx(ctx, func(tx *sql.Tx) error {
		if err := r.writeRun(ctx, tx, run, now); err != nil {
			return err
		}
		for _, event := range run.Events() {
			if _, err := tx.ExecContext(ctx, insertTransition, event.RunID, event.At.UTC(), event.From, event.Event, event.To, event.Reason); err != nil {
				return fmt.Errorf("save transition of %s: %w", run.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return domain.Run{}, err
	}
	return run.Persisted(), nil
}

func (r SQLRunRepository) writeRun(ctx context.Context, tx *sql.Tx, run domain.Run, now time.Time) error {
	next := run.Version + 1
	if run.Version == 0 {
		_, err := tx.ExecContext(ctx, insertRun, run.ID, run.Key.Repo, run.Key.Worktree, run.Prompt, run.Track, run.Phase,
			run.ReviewRound, run.LiveRound, run.StopBlocks, run.BlockedFrom, run.Reason, next, now, now)
		if r.db.Dialect().IsUniqueViolation(err) {
			return fmt.Errorf("%w: %s", domain.ErrActiveRunExists, run.ID)
		}
		return err
	}
	result, err := tx.ExecContext(ctx, updateRun, run.Track, run.Phase, run.ReviewRound, run.LiveRound, run.StopBlocks,
		run.BlockedFrom, run.Reason, next, now, run.ID, run.Version)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if changed == 0 {
		return fmt.Errorf("%w: %s at version %d", domain.ErrStaleRun, run.ID, run.Version)
	}
	return nil
}

func (r SQLRunRepository) one(ctx context.Context, query string, args ...any) (domain.Run, error) {
	var run domain.Run
	err := r.db.QueryRowContext(ctx, query, args...).Scan(&run.ID, &run.Key.Repo, &run.Key.Worktree, &run.Prompt, &run.Track, &run.Phase,
		&run.ReviewRound, &run.LiveRound, &run.StopBlocks, &run.BlockedFrom, &run.Reason, &run.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return run, err
}
