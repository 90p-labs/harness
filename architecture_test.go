package harness

import (
	"go/build"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	sourceRoot          = "internal"
	layerDomain         = "domain"
	layerApplication    = "application"
	layerInfrastructure = "infrastructure"
	otelSDKPrefix       = "go.opentelemetry.io/otel/sdk"
)

func TestLayerImports(t *testing.T) {
	err := filepath.WalkDir(sourceRoot, func(dir string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.IsDir() {
			return err
		}
		pkg, err := build.ImportDir(dir, 0)
		if _, empty := err.(*build.NoGoError); empty {
			return nil
		}
		if err != nil {
			return err
		}
		for _, imported := range pkg.Imports {
			if problem := layerProblem(dir, imported); problem != "" {
				t.Errorf("%s imports %s: %s", dir, imported, problem)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func layerProblem(dir, imported string) string {
	segments := strings.Split(filepath.ToSlash(dir), "/")
	switch {
	case slices.Contains(segments, layerDomain) && !isStandardLibrary(imported):
		return "a domain package imports only the standard library"
	case slices.Contains(segments, layerApplication) && strings.HasPrefix(imported, otelSDKPrefix):
		return "an application package imports no OpenTelemetry SDK"
	case slices.Contains(segments, layerApplication) && slices.Contains(strings.Split(imported, "/"), layerInfrastructure):
		return "an application package imports no infrastructure package"
	}
	return ""
}

func isStandardLibrary(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
