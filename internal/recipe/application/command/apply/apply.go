package apply

import (
	"bytes"
	"errors"
	"fmt"
	"path"

	"github.com/90p-labs/harness/internal/recipe/domain"
)

var ErrConflict = errors.New("recipe path conflicts with the target")

type Command struct {
	Name   domain.Name
	Target string
	DryRun bool
}

// Result lists one Change for each file, in recipe order. In a dry run,
// Wrote means the file would be written.
type Result struct {
	Recipe  domain.Recipe
	Changes []domain.Change
}

type Handler struct {
	catalogue domain.Catalogue
	open      domain.Opener
}

func NewHandler(catalogue domain.Catalogue, open domain.Opener) Handler {
	return Handler{catalogue: catalogue, open: open}
}

type step struct {
	change domain.Change
	data   []byte
}

// Handle plans every path, then writes the missing files; a conflict writes
// nothing. A failed write returns the changes already made.
func (h Handler) Handle(cmd Command) (Result, error) {
	recipe, err := h.catalogue.Get(cmd.Name)
	if err != nil {
		return Result{}, err
	}
	space := h.open(cmd.Target)

	steps := make([]step, 0, len(recipe.Files))
	for _, file := range recipe.Files {
		planned, err := h.plan(space, recipe, file)
		if err != nil {
			return Result{}, err
		}
		steps = append(steps, planned)
	}

	result := Result{Recipe: recipe, Changes: make([]domain.Change, 0, len(steps))}
	for _, planned := range steps {
		if planned.change.Action == domain.Wrote && !cmd.DryRun {
			if err := space.Write(planned.change.Path, planned.data); err != nil {
				return result, fmt.Errorf("write %s: %w", planned.change.Path, err)
			}
		}
		result.Changes = append(result.Changes, planned.change)
	}
	return result, nil
}

func (h Handler) plan(space domain.Workspace, recipe domain.Recipe, file string) (step, error) {
	if err := checkParents(space, file); err != nil {
		return step{}, err
	}
	entry, err := space.Lookup(file)
	if err != nil {
		return step{}, fmt.Errorf("look up %s: %w", file, err)
	}
	switch entry.Kind {
	case domain.Folder:
		return step{}, fmt.Errorf("%w: %s is a folder", ErrConflict, file)
	case domain.Symlink:
		return keep(file), nil
	case domain.Missing:
		data, err := h.read(recipe, file)
		if err != nil {
			return step{}, err
		}
		return step{change: domain.Change{Path: file, Action: domain.Wrote}, data: data}, nil
	default:
		want, err := h.read(recipe, file)
		if err != nil {
			return step{}, err
		}
		if bytes.Equal(entry.Data, want) {
			return keep(file), nil
		}
		return step{change: domain.Change{Path: file, Action: domain.Differs}}, nil
	}
}

func keep(file string) step {
	return step{change: domain.Change{Path: file, Action: domain.Kept}}
}

func (h Handler) read(recipe domain.Recipe, file string) ([]byte, error) {
	data, err := h.catalogue.Read(recipe.Name, file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	return data, nil
}

// checkParents fails when a parent is not a folder, a symlink included. It
// stops at the first missing parent.
func checkParents(space domain.Workspace, file string) error {
	var parents []string
	for dir := path.Dir(file); dir != "."; dir = path.Dir(dir) {
		parents = append([]string{dir}, parents...)
	}
	for _, dir := range parents {
		entry, err := space.Lookup(dir)
		if err != nil {
			return fmt.Errorf("look up %s: %w", dir, err)
		}
		if entry.Kind == domain.Missing {
			return nil
		}
		if entry.Kind != domain.Folder {
			return fmt.Errorf("%w: %s is not a folder, needed by %s", ErrConflict, dir, file)
		}
	}
	return nil
}
