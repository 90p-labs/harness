package domain

import (
	"context"
	"time"
)

type CommitCheck struct {
	Repo    string
	HeadSHA string
	Base    string
	Passed  bool
	At      time.Time
}

type EvidenceRepository interface {
	Find(ctx context.Context, repo, gateID, inputHash string) (Evidence, bool, error)
	Save(ctx context.Context, evidence Evidence) error
}

type CommitCheckRepository interface {
	Passed(ctx context.Context, repo, headSHA string) (bool, error)
	Save(ctx context.Context, check CommitCheck) error
}
