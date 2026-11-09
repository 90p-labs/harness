package guard

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"

	"github.com/90p-labs/harness/internal/guard/application/guardplugin"
	"github.com/90p-labs/harness/internal/guard/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const instrumentationName = "github.com/90p-labs/harness/internal/guard"

func NewPlugins(
	disabled []string,
	defaultBranch string,
	models map[shareddomain.Host]map[string][]string,
	worktree string,
	stop domain.StopDecider,
	commitChecked func(ctx context.Context, dir string) (bool, error),
	saveReview func(ctx context.Context, dir, model string) error,
	reviewModels func(ctx context.Context, dir string) ([]string, error),
) ([]shareddomain.Plugin, error) {
	resolve := git.DefaultBranch
	if defaultBranch != "" {
		resolve = func(context.Context, string) (string, error) { return defaultBranch, nil }
	}
	return guardplugin.Plugins(disabled, guardplugin.Deps{
		ReadFile:      os.ReadFile,
		DefaultBranch: resolve,
		Stop:          stop,
		CommitChecked: commitChecked,
		SaveReview:    saveReview,
		ReviewModels:  reviewModels,
		Models:        models,
		Worktree:      worktree,
		Meter:         otel.Meter(instrumentationName),
	})
}
