package dbal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"sort"
	"strconv"
)

const (
	schemaTable      = "harness_schema"
	migrationSuffix  = ".sql"
	createSchemaStmt = "CREATE TABLE IF NOT EXISTS " + schemaTable + " (module TEXT PRIMARY KEY, version INTEGER NOT NULL)"
	selectVersion    = "SELECT version FROM " + schemaTable + " WHERE module = ?"
	claimVersion     = "INSERT INTO " + schemaTable + " (module, version) VALUES (?, ?) ON CONFLICT (module) DO UPDATE SET version = excluded.version WHERE " + schemaTable + ".version < excluded.version"
)

var migrationName = regexp.MustCompile(`^(\d{4})_[A-Za-z0-9_]+\.sql$`)

type migration struct {
	version int
	file    string
}

// Migrate applies the files of one module above its stored version.
// The FS holds one folder for each engine, named by the engine.
func (db *DB) Migrate(ctx context.Context, module string, migrations fs.FS) error {
	engineFS, err := fs.Sub(migrations, string(db.dialect.Engine()))
	if err != nil {
		return fmt.Errorf("migrations of %s: %w", module, err)
	}
	pending, err := readMigrations(engineFS)
	if err != nil {
		return fmt.Errorf("migrations of %s: %w", module, err)
	}

	if _, err := db.ExecContext(ctx, createSchemaStmt); err != nil {
		return fmt.Errorf("create %s: %w", schemaTable, err)
	}
	var current int
	err = db.QueryRowContext(ctx, selectVersion, module).Scan(&current)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read version of %s: %w", module, err)
	}

	for _, m := range pending {
		if m.version <= current {
			continue
		}
		body, err := fs.ReadFile(engineFS, m.file)
		if err != nil {
			return fmt.Errorf("read %s: %w", m.file, err)
		}
		err = db.InTx(ctx, func(tx *sql.Tx) error {
			// The claim is the first statement, so it takes the write lock and sees a version that another process stored.
			claimed, err := tx.ExecContext(ctx, claimVersion, module, m.version)
			if err != nil {
				return err
			}
			if n, err := claimed.RowsAffected(); err != nil || n == 0 {
				return err
			}
			_, err = tx.ExecContext(ctx, string(body))
			return err
		})
		if err != nil {
			return fmt.Errorf("apply %s of %s: %w", m.file, module, err)
		}
	}
	return nil
}

func readMigrations(engineFS fs.FS) ([]migration, error) {
	entries, err := fs.ReadDir(engineFS, ".")
	if err != nil {
		return nil, err
	}
	seen := map[int]string{}
	found := make([]migration, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		match := migrationName.FindStringSubmatch(name)
		if entry.IsDir() || match == nil {
			return nil, fmt.Errorf("%w: %q", ErrBadMigrationName, name)
		}
		version, _ := strconv.Atoi(match[1])
		if version == 0 {
			return nil, fmt.Errorf("%w: %q has number 0", ErrBadMigrationName, name)
		}
		if other, dup := seen[version]; dup {
			return nil, fmt.Errorf("%w: %q and %q share number %d", ErrBadMigrationName, other, name, version)
		}
		seen[version] = name
		found = append(found, migration{version: version, file: name})
	}
	sort.Slice(found, func(i, j int) bool { return found[i].version < found[j].version })
	return found, nil
}
