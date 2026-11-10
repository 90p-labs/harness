package persistence

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	Module        = "run"
	migrationsDir = "migrations"
	emptyPRs      = "[]"

	runColumns       = "id, repo, worktree, prompt, track, phase, review_round, live_round, stop_blocks, stop_blocks_total, blocked_from, reason, pr_url, prs, version"
	selectRun        = "SELECT " + runColumns + ", created_at FROM runs"
	selectRunByID    = selectRun + " WHERE id = ?"
	selectActiveRun  = selectRun + " WHERE repo = ? AND worktree = ? AND phase NOT IN (?, ?)"
	selectByPhase    = selectRun + " WHERE repo = ? AND phase IN (%s) ORDER BY id"
	insertRun        = "INSERT INTO runs (" + runColumns + ", created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"
	updateRun        = "UPDATE runs SET track = ?, phase = ?, review_round = ?, live_round = ?, stop_blocks = ?, stop_blocks_total = ?, blocked_from = ?, reason = ?, pr_url = ?, prs = ?, version = ?, updated_at = ? WHERE id = ? AND version = ?"
	insertTransition = "INSERT INTO run_transitions (run_id, at, from_phase, event, to_phase, reason, rework_class) VALUES (?, ?, ?, ?, ?, ?, ?)"
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

func (r SQLRunRepository) ListByPhase(ctx context.Context, repo string, phases []domain.Phase) ([]domain.Run, error) {
	if len(phases) == 0 {
		return nil, nil
	}
	args := []any{repo}
	for _, phase := range phases {
		args = append(args, phase)
	}
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(phases)), ", ")
	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(selectByPhase, marks), args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var runs []domain.Run
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r SQLRunRepository) Save(ctx context.Context, run domain.Run) (domain.Run, error) {
	now := r.now().UTC()
	err := r.db.InTx(ctx, func(tx *sql.Tx) error {
		if err := r.writeRun(ctx, tx, run, now); err != nil {
			return err
		}
		for _, event := range run.Events() {
			if _, err := tx.ExecContext(ctx, insertTransition, event.RunID, event.At.UTC(), event.From, event.Event, event.To, event.Reason, event.Class); err != nil {
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
	prs, err := encodePRs(run.PRs)
	if err != nil {
		return err
	}
	if run.Version == 0 {
		_, err = tx.ExecContext(ctx, insertRun, run.ID, run.Key.Repo, run.Key.Worktree, run.Prompt, run.Track, run.Phase,
			run.ReviewRound, run.LiveRound, run.StopBlocks, run.StopBlocksTotal, run.BlockedFrom, run.Reason, run.PRURL, prs, next, run.StartedAt.UTC(), now)
		if r.db.Dialect().IsUniqueViolation(err) {
			return fmt.Errorf("%w: %s", domain.ErrActiveRunExists, run.ID)
		}
		return err
	}
	result, err := tx.ExecContext(ctx, updateRun, run.Track, run.Phase, run.ReviewRound, run.LiveRound, run.StopBlocks, run.StopBlocksTotal,
		run.BlockedFrom, run.Reason, run.PRURL, prs, next, now, run.ID, run.Version)
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
	run, err := scanRun(r.db.QueryRowContext(ctx, query, args...))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Run{}, domain.ErrRunNotFound
	}
	return run, err
}

func scanRun(row interface{ Scan(...any) error }) (domain.Run, error) {
	var run domain.Run
	var prs string
	err := row.Scan(&run.ID, &run.Key.Repo, &run.Key.Worktree, &run.Prompt, &run.Track, &run.Phase,
		&run.ReviewRound, &run.LiveRound, &run.StopBlocks, &run.StopBlocksTotal, &run.BlockedFrom, &run.Reason, &run.PRURL, &prs, &run.Version, &run.StartedAt)
	if err != nil {
		return domain.Run{}, err
	}
	if err := json.Unmarshal([]byte(prs), &run.PRs); err != nil {
		return domain.Run{}, fmt.Errorf("decode prs of %s: %w", run.ID, err)
	}
	if len(run.PRs) == 0 {
		run.PRs = nil
	}
	run.StartedAt = run.StartedAt.UTC()
	return run, nil
}

func encodePRs(prs []string) (string, error) {
	if len(prs) == 0 {
		return emptyPRs, nil
	}
	encoded, err := json.Marshal(prs)
	return string(encoded), err
}
