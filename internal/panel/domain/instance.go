package domain

import (
	"context"
	"errors"
)

// ErrStopped marks a panel that is not running.
var ErrStopped = errors.New("panel is stopped")

// Instance describes a running panel server.
type Instance struct {
	PID     int
	Port    int
	URL     string
	Repo    string
	Version string
}

// Lifecycle starts, stops and inspects the background panel server.
type Lifecycle interface {
	// Start runs the panel in the background. A running panel returns nil.
	// A port of 0 lets the server pick one.
	Start(ctx context.Context, port int) error
	// Stop ends a running panel. A stopped panel returns nil.
	Stop(ctx context.Context) error
	// Status returns the running panel, or ErrStopped.
	Status(ctx context.Context) (Instance, error)
}
