package domain

import (
	"fmt"
	"path/filepath"
)

type Setup struct {
	Gate
	Creates string
}

func NewSetup(id, run string, paths []string, creates string) (Setup, error) {
	if !gateIDPattern.MatchString(id) {
		return Setup{}, fmt.Errorf("%w: setup step %q: id must match %s", ErrInvalidSetup, id, gateIDPattern)
	}
	if run == "" {
		return Setup{}, fmt.Errorf("%w: setup step %q: run is required", ErrInvalidSetup, id)
	}
	if creates != "" && !filepath.IsLocal(creates) {
		return Setup{}, fmt.Errorf("%w: setup step %q: creates must be a path inside the project", ErrInvalidSetup, id)
	}
	return Setup{Gate: Gate{ID: id, Run: run, Paths: paths, Timeout: DefaultTimeout}, Creates: creates}, nil
}

func ValidateSetup(steps []Setup) error {
	seen := make(map[string]bool, len(steps))
	for _, step := range steps {
		if seen[step.ID] {
			return fmt.Errorf("%w: setup step %q: duplicate id", ErrInvalidSetup, step.ID)
		}
		seen[step.ID] = true
	}
	return nil
}
