package persistence

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"io/fs"
	"time"

	"github.com/90p-labs/harness/internal/host/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	Module        = "host"
	migrationsDir = "migrations"

	insertMark     = "INSERT INTO agent_marks (run_id, kind, id, started_at) VALUES (?, ?, ?, ?) ON CONFLICT (run_id, kind, id) DO NOTHING"
	insertToolMark = "INSERT INTO agent_marks (run_id, kind, id, session_id, tool, started_at) VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT (run_id, kind, id) DO NOTHING"

	deleteSessionMarks = "DELETE FROM agent_marks WHERE run_id = ? AND kind = ? AND session_id = ? RETURNING id, session_id, tool, started_at"
	deleteMark         = "DELETE FROM agent_marks WHERE run_id = ? AND kind = ? AND id = ? RETURNING started_at"
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

type SQLAgentMarks struct {
	db *dbal.DB
}

func NewSQLAgentMarks(db *dbal.DB) SQLAgentMarks {
	return SQLAgentMarks{db: db}
}

func (r SQLAgentMarks) Open(ctx context.Context, runID string, kind domain.MarkKind, id string, at time.Time) error {
	_, err := r.db.ExecContext(ctx, insertMark, runID, kind, id, at.UTC())
	return err
}

func (r SQLAgentMarks) OpenTool(ctx context.Context, runID string, mark domain.ToolMark) error {
	_, err := r.db.ExecContext(ctx, insertToolMark, runID, domain.MarkTool, mark.ID, mark.SessionID, mark.Tool, mark.StartedAt.UTC())
	return err
}

func (r SQLAgentMarks) CloseSession(ctx context.Context, runID, sessionID string) ([]domain.ToolMark, error) {
	rows, err := r.db.QueryContext(ctx, deleteSessionMarks, runID, domain.MarkTool, sessionID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var closed []domain.ToolMark
	for rows.Next() {
		var mark domain.ToolMark
		if err := rows.Scan(&mark.ID, &mark.SessionID, &mark.Tool, &mark.StartedAt); err != nil {
			return nil, err
		}
		closed = append(closed, mark)
	}
	return closed, rows.Err()
}

func (r SQLAgentMarks) Close(ctx context.Context, runID string, kind domain.MarkKind, id string) (time.Time, bool, error) {
	var started time.Time
	err := r.db.QueryRowContext(ctx, deleteMark, runID, kind, id).Scan(&started)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return started, true, nil
}
