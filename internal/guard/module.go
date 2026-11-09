package guard

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"

	"github.com/jorge07/harness/internal/guard/application/guardplugin"
	"github.com/jorge07/harness/internal/guard/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
	"github.com/jorge07/harness/internal/shared/infrastructure/git"
)

const instrumentationName = "github.com/jorge07/harness/internal/guard"

func NewPlugins(
	disabled []string,
	defaultBranch string,
	models map[shareddomain.Host]map[string][]string,
	worktree string,
	stop domain.StopDecider,
	commitChecked func(ctx context.Context, dir string) (bool, error),
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
		Models:        models,
		Worktree:      worktree,
		Meter:         otel.Meter(instrumentationName),
	})
}
