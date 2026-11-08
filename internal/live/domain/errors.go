package domain

import "errors"

var (
	ErrArchiveExists   = errors.New("archive exists")
	ErrReleaseNotFound = errors.New("release not found")
	ErrStaleRun        = errors.New("stale run")
)
