// Package repository locates and reads checkout assets for tests in any project folder.
package repository

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const moduleDeclaration = "module github.com/90p-labs/harness"

// Root finds this repository's module root without changing the working directory.
func Root(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(raw), "\n") {
			if strings.TrimSpace(line) == moduleDeclaration {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("cannot find repository root with %q", moduleDeclaration)
		}
		dir = parent
	}
}

// Path resolves a repository-relative path, or preserves an already absolute path.
func Path(t testing.TB, parts ...string) string {
	t.Helper()
	name := filepath.Join(parts...)
	if filepath.IsAbs(name) {
		return name
	}
	return filepath.Join(Root(t), name)
}

// Relative keeps repository asset identifiers independent of the checkout location.
func Relative(t testing.TB, name string) string {
	t.Helper()
	if !filepath.IsAbs(name) {
		return filepath.ToSlash(name)
	}
	name, err := filepath.Rel(Root(t), name)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.ToSlash(name)
}

// Glob returns matching asset identifiers relative to the repository root.
func Glob(t testing.TB, pattern string) ([]string, error) {
	t.Helper()
	paths, err := filepath.Glob(Path(t, pattern))
	for i, path := range paths {
		paths[i] = Relative(t, path)
	}
	return paths, err
}
