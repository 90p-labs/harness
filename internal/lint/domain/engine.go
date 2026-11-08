package domain

import (
	"context"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	LintDir     = ".harness-lint"
	VersionFile = "VERSION"
)

type Engine interface {
	Name() EngineName
	Prepare(dir string, rules []Rule) error
	Run(ctx context.Context, dir string, rules []Rule) ([]shareddomain.Finding, error)
}
