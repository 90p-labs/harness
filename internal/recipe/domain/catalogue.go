package domain

import "errors"

var ErrUnknownRecipe = errors.New("unknown recipe")

// Catalogue is the port to the recipes that ship with the binary.
type Catalogue interface {
	// List returns the recipes sorted by name.
	List() []Recipe
	// Get returns ErrUnknownRecipe when no recipe has the name.
	Get(name Name) (Recipe, error)
	// Read returns the bytes of a file that the recipe lists.
	Read(name Name, path string) ([]byte, error)
}
