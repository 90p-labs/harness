// Package status holds the query that reads the background panel.
package status

import (
	"context"

	"github.com/90p-labs/harness/internal/panel/domain"
)

// Query asks for the running panel.
type Query struct{}

// Handler reads the panel through the lifecycle port.
type Handler struct {
	lifecycle domain.Lifecycle
}

func NewHandler(lifecycle domain.Lifecycle) Handler {
	return Handler{lifecycle: lifecycle}
}

// Handle returns the running panel, or domain.ErrStopped.
func (h Handler) Handle(ctx context.Context, _ Query) (domain.Instance, error) {
	return h.lifecycle.Status(ctx)
}
