// Package catalogue loads the recipes from a file system, such as examples.FS.
package catalogue

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/90p-labs/harness/internal/recipe/domain"
)

const (
	recipeFile = "recipe.yaml"
	guideFile  = "guide.md"
)

// document is the format of a recipe.yaml.
type document struct {
	Name     string   `yaml:"name"`
	Summary  string   `yaml:"summary"`
	Files    []string `yaml:"files"`
	Scaffold []string `yaml:"scaffold"`
	Next     []string `yaml:"next"`
}

type entry struct {
	recipe domain.Recipe
	folder string
}

// Catalogue implements domain.Catalogue over a file system.
type Catalogue struct {
	fsys    fs.FS
	entries map[domain.Name]entry
	order   []domain.Name
}

var _ domain.Catalogue = (*Catalogue)(nil)

// New reads each top-level folder of fsys that holds a recipe.yaml. It fails on an
// invalid recipe, a duplicate name or a listed file that is missing.
func New(fsys fs.FS) (*Catalogue, error) {
	folders, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("read recipe folders: %w", err)
	}
	c := &Catalogue{fsys: fsys, entries: map[domain.Name]entry{}}
	for _, folder := range folders {
		if !folder.IsDir() {
			continue
		}
		recipe, found, err := load(fsys, folder.Name())
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if _, dup := c.entries[recipe.Name]; dup {
			return nil, fmt.Errorf("recipe %s is defined twice, again in folder %s", recipe.Name, folder.Name())
		}
		c.entries[recipe.Name] = entry{recipe: recipe, folder: folder.Name()}
		c.order = append(c.order, recipe.Name)
	}
	slices.Sort(c.order)
	return c, nil
}

func load(fsys fs.FS, folder string) (domain.Recipe, bool, error) {
	data, err := fs.ReadFile(fsys, path.Join(folder, recipeFile))
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Recipe{}, false, nil
	}
	if err != nil {
		return domain.Recipe{}, false, fmt.Errorf("read %s/%s: %w", folder, recipeFile, err)
	}
	var doc document
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&doc); err != nil && !errors.Is(err, io.EOF) {
		return domain.Recipe{}, false, fmt.Errorf("parse %s/%s: %w", folder, recipeFile, err)
	}
	recipe := domain.Recipe{
		Name:     domain.Name(doc.Name),
		Summary:  doc.Summary,
		Files:    doc.Files,
		Scaffold: doc.Scaffold,
		Next:     doc.Next,
	}
	if err := recipe.Validate(); err != nil {
		return domain.Recipe{}, false, fmt.Errorf("%s/%s: %w", folder, recipeFile, err)
	}
	for _, file := range recipe.Files {
		info, err := fs.Stat(fsys, path.Join(folder, file))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return domain.Recipe{}, false, fmt.Errorf("stat %s in %s: %w", file, folder, err)
		}
		if err != nil || !info.Mode().IsRegular() {
			return domain.Recipe{}, false, fmt.Errorf("recipe %s lists %s, which is not a file in %s", recipe.Name, file, folder)
		}
	}
	guide, err := fs.ReadFile(fsys, path.Join(folder, guideFile))
	switch {
	case err == nil:
		recipe.Guide = string(guide)
	case !errors.Is(err, fs.ErrNotExist):
		return domain.Recipe{}, false, fmt.Errorf("read %s/%s: %w", folder, guideFile, err)
	}
	return recipe, true, nil
}

// List returns the recipes sorted by name.
func (c *Catalogue) List() []domain.Recipe {
	recipes := make([]domain.Recipe, 0, len(c.order))
	for _, name := range c.order {
		recipes = append(recipes, clone(c.entries[name].recipe))
	}
	return recipes
}

// clone copies the slices, so a caller cannot change the catalogue.
func clone(r domain.Recipe) domain.Recipe {
	r.Files = slices.Clone(r.Files)
	r.Scaffold = slices.Clone(r.Scaffold)
	r.Next = slices.Clone(r.Next)
	return r
}

func (c *Catalogue) Get(name domain.Name) (domain.Recipe, error) {
	e, ok := c.entries[name]
	if !ok {
		return domain.Recipe{}, fmt.Errorf("%w: %q", domain.ErrUnknownRecipe, name)
	}
	return clone(e.recipe), nil
}

// Read returns a listed file. It refuses a path that the recipe does not list,
// so guide.md and recipe.yaml never reach apply.
func (c *Catalogue) Read(name domain.Name, file string) ([]byte, error) {
	e, ok := c.entries[name]
	if !ok {
		return nil, fmt.Errorf("%w: %q", domain.ErrUnknownRecipe, name)
	}
	if !slices.Contains(e.recipe.Files, file) {
		return nil, fmt.Errorf("recipe %s does not list %s", name, file)
	}
	return fs.ReadFile(c.fsys, path.Join(e.folder, file))
}
