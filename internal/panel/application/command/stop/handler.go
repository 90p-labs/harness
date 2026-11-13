// Package stop holds the command that ends the background panel.
package stop

import (
	"context"

	"github.com/90p-labs/harness/internal/panel/domain"
)

// Command stops the panel.
type Command struct{}

// Handler stops the panel through the lifecycle port.
type Handler struct {
	lifecycle domain.Lifecycle
}

func NewHandler(lifecycle domain.Lifecycle) Handler {
	return Handler{lifecycle: lifecycle}
}

// Handle stops the panel. A stopped panel returns nil.
func (h Handler) Handle(ctx context.Context, _ Command) error {
	return h.lifecycle.Stop(ctx)
}
