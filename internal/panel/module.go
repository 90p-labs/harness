// Package panel wires the served control panel.
package panel

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"github.com/90p-labs/harness/internal/panel/application/command/start"
	"github.com/90p-labs/harness/internal/panel/application/command/stop"
	"github.com/90p-labs/harness/internal/panel/application/query/board"
	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/application/query/status"
	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/panel/infrastructure/fingerprint"
	panelhttp "github.com/90p-labs/harness/internal/panel/infrastructure/http"
	"github.com/90p-labs/harness/internal/panel/infrastructure/process"
	"github.com/90p-labs/harness/internal/panel/ui/cli"
)

// NewProcess builds the process adapter of the panel.
func NewProcess(cfg process.Config) *process.Process {
	return process.New(cfg)
}

// NewLifecycle returns the lifecycle commands of the cli. A start with no port
// uses the configured port.
func NewLifecycle(proc *process.Process, port int) cli.Lifecycle {
	starter := start.NewHandler(proc, port)
	stopper := stop.NewHandler(proc)
	reader := status.NewHandler(proc)
	return cli.Lifecycle{
		Start: func(ctx context.Context, port int) error {
			return starter.Handle(ctx, start.Command{Port: port})
		},
		Stop: func(ctx context.Context) error {
			return stopper.Handle(ctx, stop.Command{})
		},
		Status: func(ctx context.Context) (domain.Instance, error) {
			return reader.Handle(ctx, status.Query{})
		},
	}
}

const watchEvery = time.Second

// watcherDone runs when the watcher goroutine ends. A test replaces it to see
// that the watcher has stopped by the time Serve returns.
var watcherDone = func() {}

// NewServe returns the serve function of the command. The ui layer cannot
// import the http server, so this module starts it.
// The server holds the panel lock from before it listens until it returns, and
// publishes panel.json once the listener has its port.
func NewServe(version, repo string, runs domain.Runs, pages domain.Pages, proc *process.Process) cli.Serve {
	return func(ctx context.Context, port int, ready func(url string)) (err error) {
		claim, err := proc.Claim()
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, claim.Release()) }()
		ln, err := panelhttp.Listen(port)
		if err != nil {
			return err
		}
		// Serve closes the listener, but a failure before it runs must not leak it.
		defer ln.Close()
		listening := ln.Addr().(*net.TCPAddr)
		if err := claim.Publish(domain.Instance{
			PID: os.Getpid(), Port: listening.Port, Repo: repo, Version: version,
		}); err != nil {
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
