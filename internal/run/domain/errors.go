package domain

import "errors"

var (
	ErrIllegalTransition = errors.New("illegal transition")
	ErrRunNotFound       = errors.New("run not found")
	ErrStaleRun          = errors.New("stale run")
	ErrActiveRunExists   = errors.New("active run exists")
)
