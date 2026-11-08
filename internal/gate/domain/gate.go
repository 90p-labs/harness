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
	Needs    []string
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
	byID := make(map[string]Gate, len(gates))
	for _, gate := range gates {
		if IsCriterionGate(gate.ID) {
			return fmt.Errorf("%w: gate %q: the id prefix %s is kept for the plan criteria", ErrInvalidGate, gate.ID, CriterionGatePrefix)
		}
		if _, dup := byID[gate.ID]; dup {
			return fmt.Errorf("%w: gate %q: duplicate id", ErrInvalidGate, gate.ID)
		}
		byID[gate.ID] = gate
	}
	for _, gate := range gates {
		for _, need := range gate.Needs {
			switch _, known := byID[need]; {
			case need == gate.ID:
				return fmt.Errorf("%w: gate %q: needs itself", ErrInvalidGate, gate.ID)
			case !known:
				return fmt.Errorf("%w: gate %q: needs unknown gate %q", ErrInvalidGate, gate.ID, need)
			}
		}
	}
	return checkCycles(gates, byID)
}

type visit int

const (
	unvisited visit = iota
	visiting
	visited
)

func checkCycles(gates []Gate, byID map[string]Gate) error {
	state := make(map[string]visit, len(gates))
	var walk func(id string) bool
	walk = func(id string) bool {
		switch state[id] {
		case visiting:
			return true
		case visited:
			return false
		}
		state[id] = visiting
		for _, need := range byID[id].Needs {
			if walk(need) {
				return true
			}
		}
		state[id] = visited
		return false
	}
	for _, gate := range gates {
		if walk(gate.ID) {
			return fmt.Errorf("%w: gate %q: needs form a cycle", ErrInvalidGate, gate.ID)
		}
	}
	return nil
}

func CriterionGateID(n int) string {
	return CriterionGatePrefix + strconv.Itoa(n)
}

func IsCriterionGate(id string) bool {
	return strings.HasPrefix(id, CriterionGatePrefix)
}
