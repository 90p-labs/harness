package domain

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound marks an unknown run or region.
	ErrNotFound = errors.New("not found")
	// ErrRegionNotFound marks a region name that no page registers. It wraps ErrNotFound.
	ErrRegionNotFound = fmt.Errorf("region %w", ErrNotFound)
	// ErrNoPlan marks a run with no plan.md.
	ErrNoPlan = errors.New("run has no plan")
)
