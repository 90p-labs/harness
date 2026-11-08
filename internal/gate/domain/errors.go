package domain

import "errors"

var (
	ErrInvalidGate = errors.New("invalid gate")
	ErrGatesExist  = errors.New("gates exist")
	ErrCommitRule  = errors.New("commit rule broken")
	ErrNoCommits   = errors.New("no commit to check in the range")
	ErrNoGates     = errors.New("no gate is set, run harness gates init")
)
