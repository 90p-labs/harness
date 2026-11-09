package host

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"

	"github.com/90p-labs/harness/internal/host/application/command/handlehook"
	hostdomain "github.com/90p-labs/harness/internal/host/domain"
	"github.com/90p-labs/harness/internal/host/infrastructure/hookcodec"
	"github.com/90p-labs/harness/internal/host/infrastructure/install"
	"github.com/90p-labs/harness/internal/host/ui/cli"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const instrumentationName = "github.com/90p-labs/harness/internal/host"

func codecs() []hostdomain.Codec {
	return []hostdomain.Codec{hookcodec.NewClaude(), hookcodec.NewCodex()}
}

func NewHookHandler(connect handlehook.Connect) handlehook.Handler {
	return handlehook.NewHandler(codecs(), connect, otel.Tracer(instrumentationName), otel.Meter(instrumentationName))
}

func NewHostConnect(worktree func(ctx context.Context) (string, error)) cli.HostConnect {
	return func(context.Context) (cli.HostHandlers, error) {
		return cli.HostHandlers{
			Install: func(ctx context.Context, host shareddomain.Host) (string, error) {
				root, err := worktree(ctx)
				if err != nil {
					return "", err
				}
				if host == "" {
					if host, err = hostdomain.OneHost(install.DetectHosts(root, os.Getenv)); err != nil {
						return "", err
					}
				}
				return install.Install(root, host)
			},
			Codecs: codecs(),
		}, nil
	}
}
