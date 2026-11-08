package dbal

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const dsnSchemeSeparator = ":"

type DB struct {
	*sql.DB
	dialect Dialect
}

func Open(ctx context.Context, dsn, baseDir string) (*DB, error) {
	scheme, path, _ := strings.Cut(dsn, dsnSchemeSeparator)

	var dialect Dialect
	switch Engine(scheme) {
	case EngineSQLite:
		dialect = newSQLite(baseDir)
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownEngine, scheme)
	}

	conn, err := sql.Open(dialect.DriverName(), dialect.DriverDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", scheme, err)
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open %s database: %w", scheme, err)
	}
	return &DB{DB: conn, dialect: dialect}, nil
}

func (db *DB) Dialect() Dialect { return db.dialect }

// InTx commits when fn returns nil. It rolls back on an error, a panic or runtime.Goexit, and a panic goes on.
func (db *DB) InTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
