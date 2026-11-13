// Package panel wires the served control panel.
package panel

import (
	"context"

	"github.com/90p-labs/harness/internal/panel/application/query/board"
	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/domain"
	panelhttp "github.com/90p-labs/harness/internal/panel/infrastructure/http"
	"github.com/90p-labs/harness/internal/panel/ui/cli"
)

// NewServe returns the serve function of the command. The ui layer cannot
// import the http server, so this module starts it.
func NewServe(version, repo string, runs domain.Runs, pages domain.Pages) cli.Serve {
	return func(ctx context.Context, port int, ready func(url string)) error {
		ln, err := panelhttp.Listen(port)
		if err != nil {
			return err
		}
		server := panelhttp.New(panelhttp.Config{
			Version: version,
			Repo:    repo,
			Board:   board.NewHandler(runs),
			Page:    page.NewHandler(runs, pages),
		})
		ready("http://" + ln.Addr().String() + "/")
		return server.Serve(ctx, ln)
	}
}
