package domain

import (
	"context"
	"time"
)

type MergeState string

const (
	MergeOpen   MergeState = "open"
	MergeMerged MergeState = "merged"
	MergeClosed MergeState = "closed"
)

type Merge struct {
	State MergeState
	SHA   string
	At    time.Time
}

type DeployState string

const (
	DeployPending DeployState = "pending"
	DeploySuccess DeployState = "success"
	DeployFailed  DeployState = "failed"
)

type Deployment struct {
	State DeployState
	At    time.Time
}

type GitHub interface {
	Merge(ctx context.Context, prURL string) (Merge, error)
	Deployment(ctx context.Context, prURL, sha, environment string) (Deployment, error)
}
