package dbal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"testing/fstest"
	"time"
)

const testModule = "demo"

func openTemp(t *testing.T) *DB {
	t.Helper()
	db, err := Open(context.Background(), "sqlite:state.db", t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func migrationsOf(files map[string]string) fstest.MapFS {
	fsys := fstest.MapFS{}
	for name, body := range files {
		fsys[string(EngineSQLite)+"/"+name] = &fstest.MapFile{Data: []byte(body)}
	}
	return fsys
}

func tableNames(t *testing.T, db *DB) []string {
	t.Helper()
	rows, err := db.QueryContext(context.Background(), "SELECT name FROM sqlite_master WHERE type = 'table' ORDER BY name")
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, name)
	}
	return names
}

func storedVersion(t *testing.T, db *DB) int {
	t.Helper()
	var version int
	err := db.QueryRowContext(context.Background(), "SELECT version FROM harness_schema WHERE module = ?", testModule).Scan(&version)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	return version
}

func TestOpenUnknownEngine(t *testing.T) {
	for _, dsn := range []string{"mysql:host/db", "no-scheme", ""} {
		t.Run(dsn, func(t *testing.T) {
			db, err := Open(context.Background(), dsn, t.TempDir())
			if !errors.Is(err, ErrUnknownEngine) {
				t.Fatalf("err = %v, want ErrUnknownEngine", err)
			}
			if db != nil {
				t.Fatal("db must be nil on error")
			}
		})
	}
}

func TestOpenSQLitePragmas(t *testing.T) {
	base := t.TempDir()
	db, err := Open(context.Background(), "sqlite:nested/dir/state.db", base)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := os.Stat(filepath.Join(base, "nested", "dir", "state.db")); err != nil {
		t.Fatalf("database file under baseDir: %v", err)
	}
	if db.Dialect().Engine() != EngineSQLite {
		t.Fatalf("engine = %q", db.Dialect().Engine())
	}

	var journal string
	var busy, foreign int
	ctx := context.Background()
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&busy); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&foreign); err != nil {
		t.Fatal(err)
	}
	if journal != "wal" || busy != 5000 || foreign != 1 {
		t.Fatalf("journal=%q busy=%d foreign_keys=%d", journal, busy, foreign)
	}
}

func TestOpenSQLiteSpecialPath(t *testing.T) {
	for _, dir := range []string{"my repo", "my#repo", "my?repo", "my%41repo"} {
		t.Run(dir, func(t *testing.T) {
			base := filepath.Join(t.TempDir(), dir)
			db, err := Open(context.Background(), "sqlite:state.db", base)
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			t.Cleanup(func() { _ = db.Close() })

			if _, err := os.Stat(filepath.Join(base, "state.db")); err != nil {
				t.Fatalf("database file under baseDir: %v", err)
			}
			var journal string
			if err := db.QueryRowContext(context.Background(), "PRAGMA journal_mode").Scan(&journal); err != nil {
				t.Fatal(err)
			}
			if journal != "wal" {
				t.Fatalf("journal = %q, want wal", journal)
			}
		})
	}
}

func TestMigrateAppliesOnlyGap(t *testing.T) {
	ctx := context.Background()
	db := openTemp(t)

	first := migrationsOf(map[string]string{
		"0001_create.sql": "CREATE TABLE log (n INTEGER NOT NULL);",
		"0002_one.sql":    "INSERT INTO log VALUES (2);",
	})
	if err := db.Migrate(ctx, testModule, first); err != nil {
		t.Fatalf("first Migrate: %v", err)
	}
	if got := storedVersion(t, db); got != 2 {
		t.Fatalf("version = %d, want 2", got)
	}

	second := migrationsOf(map[string]string{
		"0001_create.sql": "CREATE TABLE log (n INTEGER NOT NULL);",
		"0002_one.sql":    "INSERT INTO log VALUES (2);",
		"0010_late.sql":   "INSERT INTO log VALUES (10);",
		"0003_mid.sql":    "INSERT INTO log VALUES (3);",
	})
	if err := db.Migrate(ctx, testModule, second); err != nil {
		t.Fatalf("second Migrate: %v", err)
	}
	if got := storedVersion(t, db); got != 10 {
		t.Fatalf("version = %d, want 10", got)
	}

	rows, err := db.QueryContext(ctx, "SELECT n FROM log ORDER BY rowid")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []int
	for rows.Next() {
		var n int
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		got = append(got, n)
	}
	if fmt.Sprint(got) != "[2 3 10]" {
		t.Fatalf("log = %v, want [2 3 10]", got)
	}

	if err := db.Migrate(ctx, testModule, second); err != nil {
		t.Fatalf("idempotent Migrate: %v", err)
	}
}

type gatedFS struct {
	fs.ReadDirFS
	reached chan struct{}
	gate    chan struct{}
}

func (g gatedFS) Open(name string) (fs.File, error) {
	if path.Ext(name) == migrationSuffix {
		close(g.reached)
		<-g.gate
	}
	return g.ReadDirFS.Open(name)
}

func TestMigrateRacesAnotherProcess(t *testing.T) {
	ctx := context.Background()
	base := t.TempDir()
	var slow, fast *DB
	for _, db := range []**DB{&slow, &fast} {
		opened, err := Open(ctx, "sqlite:state.db", base)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = opened.Close() })
		*db = opened
	}
	files := migrationsOf(map[string]string{"0001_create.sql": "CREATE TABLE log (n INTEGER NOT NULL);"})

	slowFS := gatedFS{ReadDirFS: files, reached: make(chan struct{}), gate: make(chan struct{})}
	slowDone := make(chan error)
	go func() { slowDone <- slow.Migrate(ctx, testModule, slowFS) }()
	<-slowFS.reached
	if err := fast.Migrate(ctx, testModule, files); err != nil {
		t.Fatalf("fast Migrate: %v", err)
	}
	close(slowFS.gate)

	if err := <-slowDone; err != nil {
		t.Fatalf("slow Migrate after a finished one: %v", err)
	}
	if got := storedVersion(t, fast); got != 1 {
		t.Fatalf("version = %d, want 1", got)
	}
}

func TestMigrateFailedFileKeepsPreviousVersion(t *testing.T) {
	ctx := context.Background()
	db := openTemp(t)

	err := db.Migrate(ctx, testModule, migrationsOf(map[string]string{
		"0001_ok.sql":  "CREATE TABLE ok (n INTEGER);",
		"0002_bad.sql": "CREATE TABLE half (n INTEGER); INSERT INTO missing VALUES (1);",
	}))
	if err == nil {
		t.Fatal("want error from the broken file")
	}
	if got := storedVersion(t, db); got != 1 {
		t.Fatalf("version = %d, want 1", got)
	}
	for _, name := range tableNames(t, db) {
		if name == "half" {
			t.Fatal("failed migration left a table behind")
		}
	}
}

func TestMigrateBadFileName(t *testing.T) {
	ctx := context.Background()
	valid := "CREATE TABLE ok (n INTEGER);"
	cases := map[string]fstest.MapFS{
		"bad name": migrationsOf(map[string]string{
			"0001_ok.sql": valid,
			"create.sql":  valid,
		}),
		"duplicate number": migrationsOf(map[string]string{
			"0001_ok.sql":    valid,
			"0001_other.sql": valid,
		}),
		"non sql entry": migrationsOf(map[string]string{
			"0001_ok.sql": valid,
			"README.md":   "notes",
		}),
		"number zero": migrationsOf(map[string]string{
			"0000_zero.sql": valid,
			"0001_ok.sql":   valid,
		}),
		"subfolder": migrationsOf(map[string]string{
			"0001_ok.sql":         valid,
			"nested/0002_sub.sql": valid,
		}),
	}
	for name, fsys := range cases {
		t.Run(name, func(t *testing.T) {
			db := openTemp(t)
			before := tableNames(t, db)

			err := db.Migrate(ctx, testModule, fsys)
			if !errors.Is(err, ErrBadMigrationName) {
				t.Fatalf("err = %v, want ErrBadMigrationName", err)
			}
			if after := tableNames(t, db); fmt.Sprint(after) != fmt.Sprint(before) {
				t.Fatalf("schema changed: %v -> %v", before, after)
			}
		})
	}
}

func TestInTxRollback(t *testing.T) {
	ctx := context.Background()
	setup := func(t *testing.T) *DB {
		db := openTemp(t)
		if _, err := db.ExecContext(ctx, "CREATE TABLE item (n INTEGER)"); err != nil {
			t.Fatal(err)
		}
		return db
	}
	count := func(t *testing.T, db *DB) int {
		var n int
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM item").Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	insert := func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO item VALUES (1)")
		return err
	}

	t.Run("commit", func(t *testing.T) {
		db := setup(t)
		if err := db.InTx(ctx, insert); err != nil {
			t.Fatal(err)
		}
		if got := count(t, db); got != 1 {
			t.Fatalf("rows = %d, want 1", got)
		}
	})

	t.Run("error", func(t *testing.T) {
		db := setup(t)
		boom := errors.New("boom")
		err := db.InTx(ctx, func(tx *sql.Tx) error {
			if err := insert(tx); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatalf("err = %v, want boom", err)
		}
		if got := count(t, db); got != 0 {
			t.Fatalf("rows = %d, want 0", got)
		}
	})

	t.Run("panic", func(t *testing.T) {
		db := setup(t)
		var recovered any
		func() {
			defer func() { recovered = recover() }()
			_ = db.InTx(ctx, func(tx *sql.Tx) error {
				if err := insert(tx); err != nil {
					return err
				}
				panic("kaboom")
			})
		}()
		if recovered != "kaboom" {
			t.Fatalf("recovered = %v, want the original panic", recovered)
		}
		if got := count(t, db); got != 0 {
			t.Fatalf("rows = %d, want 0", got)
		}
	})

	t.Run("goexit", func(t *testing.T) {
		db := setup(t)
		done := make(chan struct{})
		go func() {
			defer close(done)
			_ = db.InTx(ctx, func(tx *sql.Tx) error {
				_ = insert(tx)
				runtime.Goexit()
				return nil
			})
		}()
		<-done

		writeCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		if _, err := db.ExecContext(writeCtx, "INSERT INTO item VALUES (2)"); err != nil {
			t.Fatalf("write after goexit: %v", err)
		}
		if got := count(t, db); got != 1 {
			t.Fatalf("rows = %d, want 1", got)
		}
	})
}

func TestTimesUseSQLiteFormat(t *testing.T) {
	ctx := context.Background()
	db := openTemp(t)
	if _, err := db.ExecContext(ctx, "CREATE TABLE stamp (at TIMESTAMP NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO stamp VALUES (?)", time.Date(2026, 11, 8, 11, 30, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}

	var parsed sql.NullString
	if err := db.QueryRowContext(ctx, "SELECT datetime(at) FROM stamp").Scan(&parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.String != "2026-11-08 11:30:00" {
		t.Fatalf("datetime(at) = %v, want 2026-11-08 11:30:00", parsed)
	}
}

func TestIsUniqueViolation(t *testing.T) {
	ctx := context.Background()
	db := openTemp(t)
	setup := "CREATE TABLE u (k TEXT PRIMARY KEY, v TEXT NOT NULL); CREATE UNIQUE INDEX u_v ON u (v) WHERE v <> 'free';"
	if _, err := db.ExecContext(ctx, setup); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, "INSERT INTO u VALUES ('a', 'x')"); err != nil {
		t.Fatal(err)
	}
	_, dupKey := db.ExecContext(ctx, "INSERT INTO u VALUES ('a', 'y')")
	_, dup := db.ExecContext(ctx, "INSERT INTO u VALUES ('b', 'x')")
	_, notNull := db.ExecContext(ctx, "INSERT INTO u VALUES ('c', NULL)")

	d := db.Dialect()
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"duplicate primary key", dupKey, false},
		{"duplicate unique index", dup, true},
		{"wrapped duplicate", fmt.Errorf("save: %w", dup), true},
		{"not null violation", notNull, false},
		{"plain error", errors.New("x"), false},
		{"nil", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := d.IsUniqueViolation(c.err); got != c.want {
				t.Fatalf("IsUniqueViolation = %v, want %v (err %v)", got, c.want, c.err)
			}
		})
	}
}
