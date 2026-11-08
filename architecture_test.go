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
	modulePath          = "github.com/jorge07/harness"
	sharedDomainImport  = modulePath + "/internal/shared/domain"
)

func TestLayerImportsRules(t *testing.T) {
	cases := []struct {
		name     string
		dir      string
		imported string
		allowed  bool
	}{
		{"domain imports the standard library", "internal/run/domain", "encoding/json", true},
		{"domain imports the shared domain", "internal/plugin/domain", sharedDomainImport, true},
		{"domain imports another domain", "internal/plugin/domain", modulePath + "/internal/run/domain", false},
		{"domain imports shared infrastructure", "internal/plugin/domain", modulePath + "/internal/shared/infrastructure/config", false},
		{"domain imports a sub package of the shared domain", "internal/plugin/domain", sharedDomainImport + "/extra", false},
		{"domain imports a third party module", "internal/plugin/domain", "github.com/spf13/viper", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := layerProblem(tc.dir, tc.imported) == ""; got != tc.allowed {
				t.Errorf("layerProblem(%q, %q) allowed = %v, want %v", tc.dir, tc.imported, got, tc.allowed)
			}
		})
	}
}

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
	case slices.Contains(segments, layerDomain) && !isStandardLibrary(imported) && imported != sharedDomainImport:
		return "a domain package imports only the standard library and the shared domain"
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
