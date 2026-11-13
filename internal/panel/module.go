// Package panel wires the served control panel.
package panel

import (
	"context"
	"sync"
	"time"

	"github.com/90p-labs/harness/internal/panel/application/query/board"
	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/panel/infrastructure/fingerprint"
	panelhttp "github.com/90p-labs/harness/internal/panel/infrastructure/http"
	"github.com/90p-labs/harness/internal/panel/ui/cli"
)

const watchEvery = time.Second

// watcherDone runs when the watcher goroutine ends. A test replaces it to see
// that the watcher has stopped by the time Serve returns.
var watcherDone = func() {}

// NewServe returns the serve function of the command. The ui layer cannot
// import the http server, so this module starts it.
func NewServe(version, repo string, runs domain.Runs, pages domain.Pages) cli.Serve {
	return func(ctx context.Context, port int, ready func(url string)) error {
		ln, err := panelhttp.Listen(port)
		if err != nil {
			return err
		}
		watcher := fingerprint.NewWatcher(runs, watchEvery)
		server := panelhttp.New(panelhttp.Config{
			Version: version,
			Repo:    repo,
			Board:   board.NewHandler(runs),
			Page:    page.NewHandler(runs, pages),
			Signals: watcher,
			Now:     time.Now,
		})

		// The watcher reads the run state, so Serve must not return while it runs.
		watchCtx, stopWatch := context.WithCancel(ctx)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer watcherDone()
			watcher.Run(watchCtx)
		}()
		defer wg.Wait()
		defer stopWatch()

		ready("http://" + ln.Addr().String() + "/")
		return server.Serve(ctx, ln)
	}
}
