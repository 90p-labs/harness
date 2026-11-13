// Package start holds the command that runs the panel in the background.
package start

import (
	"context"

	"github.com/90p-labs/harness/internal/panel/domain"
)

// DefaultPort in a Command asks for the configured port.
const DefaultPort = -1

// Command starts the panel. A Port of DefaultPort uses the configured port.
type Command struct {
	Port int
}

// Handler starts the panel through the lifecycle port.
type Handler struct {
	lifecycle domain.Lifecycle
	port      int
}

// NewHandler returns a handler that uses port when the command has no port.
func NewHandler(lifecycle domain.Lifecycle, port int) Handler {
	return Handler{lifecycle: lifecycle, port: port}
}

// Handle starts the panel. A running panel is left as it is.
func (h Handler) Handle(ctx context.Context, cmd Command) error {
	port := cmd.Port
	if port < 0 {
		port = h.port
	}
	return h.lifecycle.Start(ctx, port)
}
