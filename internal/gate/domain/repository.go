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

type SetupRepository interface {
	LastPass(ctx context.Context, repo, stepID string) (inputHash string, ok bool, err error)
	SavePass(ctx context.Context, repo, stepID, inputHash string, at time.Time) error
}

type CommitCheckRepository interface {
	Passed(ctx context.Context, repo, headSHA string) (bool, error)
	Save(ctx context.Context, check CommitCheck) error
}

type ReviewRecord struct {
	Repo    string
	HeadSHA string
	Model   string
	At      time.Time
}

type ReviewRecordRepository interface {
	Save(ctx context.Context, record ReviewRecord) error
	Models(ctx context.Context, repo, headSHA string) ([]string, error)
}
