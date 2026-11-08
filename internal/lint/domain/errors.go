package domain

import "errors"

var (
	ErrMissingPack    = errors.New("missing required pack")
	ErrUnknownPack    = errors.New("unknown pack")
	ErrStaleLintFiles = errors.New("lint files are stale, run harness lint init")
	ErrEngineMissing  = errors.New("lint engine is missing")
)
