package internal_test

import (
	"go/build"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	sourceRoot          = "internal"
	layerDomain         = "domain"
	layerApplication    = "application"
	layerInfrastructure = "infrastructure"
	layerUI             = "ui"
	otelSDKPrefix       = "go.opentelemetry.io/otel/sdk"
	netHTTPImport       = "net/http"
	modulePath          = "github.com/90p-labs/harness"
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
		{"domain imports the shared domain", "internal/run/domain", sharedDomainImport, true},
		{"domain imports another domain", "internal/run/domain", modulePath + "/internal/plan/domain", false},
		{"domain imports shared infrastructure", "internal/run/domain", modulePath + "/internal/shared/infrastructure/config", false},
		{"domain imports a sub package of the shared domain", "internal/run/domain", sharedDomainImport + "/extra", false},
		{"domain imports a third party module", "internal/run/domain", "github.com/spf13/viper", false},
		{"ui imports infrastructure", "internal/run/ui/cli", modulePath + "/internal/run/infrastructure/persistence", false},
		{"ui imports shared infrastructure", "internal/run/ui/cli", modulePath + "/internal/shared/infrastructure/config", false},
		{"ui imports an application package", "internal/run/ui/cli", modulePath + "/internal/run/application/query/status", true},
		{"domain imports net/http", "internal/panel/domain", "net/http", false},
		{"application imports net/http", "internal/panel/application/query/page", "net/http", false},
		{"infrastructure imports net/http", "internal/panel/infrastructure/http", "net/http", true},
		{"panel domain imports the standard library", "internal/panel/domain", "crypto/sha256", true},
		{"panel domain imports a plan domain", "internal/panel/domain", modulePath + "/internal/plan/domain", false},
		{"panel application imports the panel domain", "internal/panel/application/query/board", modulePath + "/internal/panel/domain", true},
		{"panel application imports infrastructure", "internal/panel/application/query/page", modulePath + "/internal/panel/infrastructure/http", false},
		{"panel ui imports the http server", "internal/panel/ui/cli", modulePath + "/internal/panel/infrastructure/http", false},
		{"panel ui imports shared infrastructure", "internal/panel/ui/cli", modulePath + "/internal/shared/infrastructure/webkit", false},
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
	err := filepath.WalkDir(repository.Path(t, sourceRoot), func(dir string, entry fs.DirEntry, err error) error {
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
			if problem := layerProblem(repository.Relative(t, dir), imported); problem != "" {
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
	case (slices.Contains(segments, layerDomain) || slices.Contains(segments, layerApplication)) && imported == netHTTPImport:
		return "a domain or application package imports no net/http"
	case slices.Contains(segments, layerDomain) && !isStandardLibrary(imported) && imported != sharedDomainImport:
		return "a domain package imports only the standard library and the shared domain"
	case slices.Contains(segments, layerApplication) && strings.HasPrefix(imported, otelSDKPrefix):
		return "an application package imports no OpenTelemetry SDK"
	case slices.Contains(segments, layerApplication) && slices.Contains(strings.Split(imported, "/"), layerInfrastructure):
		return "an application package imports no infrastructure package"
	case slices.Contains(segments, layerUI) && slices.Contains(strings.Split(imported, "/"), layerInfrastructure):
		return "a ui package imports no infrastructure package"
	}
	return ""
}

func isStandardLibrary(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}
