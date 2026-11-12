package host

import (
	"context"
	"os"
	"time"

	"go.opentelemetry.io/otel"

	"github.com/90p-labs/harness/internal/host/application/command/handlehook"
	"github.com/90p-labs/harness/internal/host/application/marksplugin"
	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	"github.com/90p-labs/harness/internal/host/infrastructure/hookcodec"
	"github.com/90p-labs/harness/internal/host/infrastructure/install"
	"github.com/90p-labs/harness/internal/host/infrastructure/persistence"
	"github.com/90p-labs/harness/internal/host/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const instrumentationName = "github.com/90p-labs/harness/internal/host"

func codecs() []hostdomain.Codec {
	return []hostdomain.Codec{hookcodec.NewClaude(), hookcodec.NewCodex()}
}

func NewHookHandler(connect handlehook.Connect, marks hostdomain.AgentMarks) handlehook.Handler {
	return handlehook.NewHandler(codecs(), connect, otel.Tracer(instrumentationName), otel.Meter(instrumentationName), marks, time.Now)
}

func NewMarksPlugin(db func(ctx context.Context) (*dbal.DB, error)) shareddomain.Plugin {
	return marksplugin.New(func(ctx context.Context, runID string) error {
		conn, err := db(ctx)
		if err != nil {
			return err
		}
		return persistence.NewSQLAgentMarks(conn).DeleteRun(ctx, runID)
	})
}

// NewHostConnect wires the host commands. The target gives the worktree and whether the Claude tool hooks match only the rule tools.
func NewHostConnect(target func(ctx context.Context) (worktree string, matched bool, err error)) cli.HostConnect {
	// resolve gives the worktree, the matched flag and the host, detected when it is empty.
	resolve := func(ctx context.Context, host shareddomain.Host) (string, bool, shareddomain.Host, error) {
		root, matched, err := target(ctx)
		if err != nil {
			return "", false, "", err
		}
		if host == "" {
			if host, err = hostdomain.OneHost(install.DetectHosts(root, os.Getenv)); err != nil {
				return "", false, "", err
			}
		}
		return root, matched, host, nil
	}
	return func(context.Context) (cli.HostHandlers, error) {
		return cli.HostHandlers{
			Install: func(ctx context.Context, host shareddomain.Host) (string, error) {
				root, matched, host, err := resolve(ctx, host)
				if err != nil {
					return "", err
				}
				return install.Install(root, host, matched)
			},
			Plugin: func(ctx context.Context, host shareddomain.Host) (bool, error) {
				root, _, host, err := resolve(ctx, host)
				if err != nil || host != shareddomain.HostClaude {
					return false, err
				}
				return install.ClaudePlugin(root, os.Getenv)
			},
			Uninstall: func(ctx context.Context, host shareddomain.Host) (string, int, error) {
				root, _, host, err := resolve(ctx, host)
				if err != nil {
					return "", 0, err
				}
				return install.Uninstall(root, host)
			},
			Codecs: codecs(),
		}, nil
	}
}
