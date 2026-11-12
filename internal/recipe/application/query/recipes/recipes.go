package recipes

import (
	"github.com/90p-labs/harness/internal/recipe/domain"
)

type Handler struct {
	catalogue domain.Catalogue
}

func NewHandler(catalogue domain.Catalogue) Handler {
	return Handler{catalogue: catalogue}
}

// Handle returns the recipes in the order of the catalogue, which lists them
// sorted by name.
func (h Handler) Handle() []domain.Recipe {
	return h.catalogue.List()
}
