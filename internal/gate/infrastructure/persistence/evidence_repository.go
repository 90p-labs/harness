package persistence

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"io/fs"
	"time"

	"github.com/jorge07/harness/internal/gate/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/dbal"
)

const (
	Module        = "gate"
	migrationsDir = "migrations"

	selectEvidence = "SELECT result, duration_ms, tail, at FROM gate_evidence WHERE repo = ? AND gate_id = ? AND input_hash = ?"
	upsertEvidence = "INSERT INTO gate_evidence (repo, gate_id, input_hash, result, duration_ms, tail, at) VALUES (?, ?, ?, ?, ?, ?, ?)" +
		" ON CONFLICT (repo, gate_id, input_hash) DO UPDATE SET result = excluded.result, duration_ms = excluded.duration_ms, tail = excluded.tail, at = excluded.at"
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

type SQLEvidenceRepository struct {
	db *dbal.DB
}

func NewSQLEvidenceRepository(db *dbal.DB) SQLEvidenceRepository {
	return SQLEvidenceRepository{db: db}
}

func (r SQLEvidenceRepository) Find(ctx context.Context, repo, gateID, inputHash string) (domain.Evidence, bool, error) {
	evidence := domain.Evidence{Repo: repo, GateID: gateID, InputHash: inputHash}
	var durationMS int64
	err := r.db.QueryRowContext(ctx, selectEvidence, repo, gateID, inputHash).Scan(&evidence.Result, &durationMS, &evidence.Tail, &evidence.At)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Evidence{}, false, nil
	}
	if err != nil {
		return domain.Evidence{}, false, err
	}
	evidence.Duration = time.Duration(durationMS) * time.Millisecond
	return evidence, true, nil
}

func (r SQLEvidenceRepository) Save(ctx context.Context, evidence domain.Evidence) error {
	_, err := r.db.ExecContext(ctx, upsertEvidence, evidence.Repo, evidence.GateID, evidence.InputHash, evidence.Result,
		evidence.Duration.Milliseconds(), evidence.Tail, evidence.At.UTC())
	return err
}
