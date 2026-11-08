package domain

import "time"

type Release struct {
	RunID       string
	PRURL       string
	MergeSHA    string
	MergedAt    time.Time
	DeployedAt  time.Time
	Environment string
}
