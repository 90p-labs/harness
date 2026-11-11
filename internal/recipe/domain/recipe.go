package domain

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
)

const (
	namePattern = `^[a-z][a-z0-9]*(-[a-z0-9]+)*$`
	gitFolder   = ".git"
	parentPart  = ".."
)

var (
	ErrInvalidName   = errors.New("invalid recipe name")
	ErrNoFiles       = errors.New("a recipe lists at least one file")
	ErrBadPath       = errors.New("recipe path must be relative, clean, outside .git and free of ..")
	ErrDuplicatePath = errors.New("recipe path is listed twice")
	ErrPathIsFolder  = errors.New("recipe path is the parent folder of another listed path")
	ErrNeverShips    = errors.New("recipe.yaml, guide.md, README.md and *_test.go never ship in a recipe")

	nameRule = regexp.MustCompile(namePattern)
)

type Name string

func (n Name) String() string { return string(n) }

type Recipe struct {
	Name     Name
	Summary  string
	Files    []string
	Scaffold []string
	Next     []string
	Guide    string
}

// Validate checks the name and every file path. The file list is not empty.
func (r Recipe) Validate() error {
	// $ in Go RE2 matches only at the end of the text, so a trailing newline fails.
	if !nameRule.MatchString(r.Name.String()) {
		return fmt.Errorf("%w: %q must match %s", ErrInvalidName, r.Name, namePattern)
	}
	if len(r.Files) == 0 {
		return fmt.Errorf("%w: recipe %s", ErrNoFiles, r.Name)
	}
	seen := make(map[string]bool, len(r.Files))
	for _, file := range r.Files {
		if !validPath(file) {
			return fmt.Errorf("%w: %q in recipe %s", ErrBadPath, file, r.Name)
		}
		if seen[file] {
			return fmt.Errorf("%w: %q in recipe %s", ErrDuplicatePath, file, r.Name)
		}
		if neverShips(file) {
			return fmt.Errorf("%w: %q in recipe %s", ErrNeverShips, file, r.Name)
		}
		seen[file] = true
	}
	return r.checkNoFolderPaths(seen)
}

// checkNoFolderPaths fails when a listed path is the parent folder of another
// listed path, because the file and the folder cannot both exist.
func (r Recipe) checkNoFolderPaths(listed map[string]bool) error {
	for _, file := range r.Files {
		for dir := path.Dir(file); dir != "."; dir = path.Dir(dir) {
			if listed[dir] {
				return fmt.Errorf("%w: %q holds %q in recipe %s", ErrPathIsFolder, dir, file, r.Name)
			}
		}
	}
	return nil
}

func neverShips(file string) bool {
	switch base := path.Base(file); base {
	case "recipe.yaml", "guide.md", "README.md":
		return true
	default:
		return strings.HasSuffix(base, "_test.go")
	}
}

func validPath(file string) bool {
	if file == "" || strings.Contains(file, `\`) || path.IsAbs(file) || path.Clean(file) != file {
		return false
	}
	parts := strings.Split(file, "/")
	for _, part := range parts {
		if part == parentPart || part == "." {
			return false
		}
	}
	// A case-insensitive file system maps .GIT to .git.
	return !strings.EqualFold(parts[0], gitFolder)
}
