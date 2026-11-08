package domain

import (
	"context"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	LintDir      = ".harness-lint"
	VersionFile  = "VERSION"
	BaselineFile = "baseline.json"
	BaselinePath = LintDir + "/" + BaselineFile

	OldBaselinePath = ".harness-baseline/lint.json"
)

type Engine interface {
	Name() EngineName
	Prepare(dir string, rules []Rule) error
	Run(ctx context.Context, dir string, rules []Rule) ([]shareddomain.Finding, error)
}

type EngineSet interface {
	Check(ctx context.Context, dir string, engine EngineName, rules []Rule) ([]shareddomain.Finding, error)
	Prepare(ctx context.Context, dir string, engine EngineName, rules []Rule) error
}

type CheckPayload struct {
	Rules []Rule   `json:"rules"`
	Files []string `json:"files"`
}
