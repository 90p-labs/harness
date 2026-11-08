package dbal

import "errors"

var (
	ErrUnknownEngine    = errors.New("unknown database engine")
	ErrBadMigrationName = errors.New("bad migration file name")
)
