package dbal

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

const (
	sqliteDriverName  = "sqlite"
	sqliteFilePrefix  = "file:"
	sqlitePragmaQuery = "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_time_format=sqlite"
	sqliteDirMode     = 0o755
)

type sqliteDialect struct {
	baseDir string
}

func newSQLite(baseDir string) sqliteDialect {
	return sqliteDialect{baseDir: baseDir}
}

func (sqliteDialect) Engine() Engine { return EngineSQLite }

func (sqliteDialect) DriverName() string { return sqliteDriverName }

func (d sqliteDialect) filePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(d.baseDir, path)
}

func (d sqliteDialect) DriverDSN(path string) string {
	path = d.filePath(path)
	// A failure here surfaces as an open error from the driver.
	_ = os.MkdirAll(filepath.Dir(path), sqliteDirMode)
	return sqliteFilePrefix + (&url.URL{Path: path}).EscapedPath() + sqlitePragmaQuery
}

func (sqliteDialect) IsUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE
}
