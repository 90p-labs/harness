package domain

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimeout      = 10 * time.Minute
	CriterionGatePrefix = "criterion-"
)

var gateIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type Gate struct {
	ID       string
	Run      string
	Paths    []string
	Timeout  time.Duration
	Baseline string
}

func NewGate(id, run string, paths []string, timeout time.Duration, baseline string) (Gate, error) {
	if !gateIDPattern.MatchString(id) {
		return Gate{}, fmt.Errorf("%w: gate %q: id must match %s", ErrInvalidGate, id, gateIDPattern)
	}
	if run == "" {
		return Gate{}, fmt.Errorf("%w: gate %q: run is required", ErrInvalidGate, id)
	}
	if timeout < 0 {
		return Gate{}, fmt.Errorf("%w: gate %q: timeout must not be negative", ErrInvalidGate, id)
	}
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	return Gate{ID: id, Run: run, Paths: paths, Timeout: timeout, Baseline: baseline}, nil
}

func ValidateGates(gates []Gate) error {
	seen := make(map[string]struct{}, len(gates))
	for _, gate := range gates {
		if _, dup := seen[gate.ID]; dup {
			return fmt.Errorf("%w: gate %q: duplicate id", ErrInvalidGate, gate.ID)
		}
		seen[gate.ID] = struct{}{}
	}
	return nil
}

func CriterionGateID(n int) string {
	return CriterionGatePrefix + strconv.Itoa(n)
}

func IsCriterionGate(id string) bool {
	return strings.HasPrefix(id, CriterionGatePrefix)
}
