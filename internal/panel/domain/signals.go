package domain

import "context"

const (
	// SignalRunsChanged tells a board subscriber that the board version changed.
	SignalRunsChanged = "runs-changed"
	// SignalRunUpdated tells a run subscriber that the run version changed.
	SignalRunUpdated = "run-updated"
)

// Signals delivers change signals to the open pages.
type Signals interface {
	// Subscribe returns a channel of signal names and a cancel func. An empty run
	// subscribes to the board. An unknown run returns ErrNotFound.
	Subscribe(ctx context.Context, run string) (<-chan string, func(), error)
}
