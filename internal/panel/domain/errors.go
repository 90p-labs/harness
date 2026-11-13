package domain

import "errors"

var (
	// ErrNotFound marks an unknown run or region.
	ErrNotFound = errors.New("not found")
	// ErrNoPlan marks a run with no plan.md.
	ErrNoPlan = errors.New("run has no plan")
)
