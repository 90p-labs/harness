package manifest_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/manifest"
)

func writeManifest(t *testing.T, content string) string {
	t.Helper()
	repo := t.TempDir()
	path := filepath.Join(repo, filepath.FromSlash(domain.ManifestFile))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return repo
}

func TestRead(t *testing.T) {
	repo := writeManifest(t, `pack: ddd
rules:
  - id: ddd/domain-purity
    canary: lint/canaries/ddd-domain-purity
  - id: ddd/file-suffix
    waived: deptrac checks dependencies, not file names.
`)

	got, err := manifest.Read(repo)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	want := domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{
		{ID: "ddd/domain-purity", Canary: "lint/canaries/ddd-domain-purity"},
		{ID: "ddd/file-suffix", Waived: "deptrac checks dependencies, not file names."},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Read = %+v, want %+v", got, want)
	}
}

func TestReadRefuses(t *testing.T) {
	t.Run("no manifest names the setup skill", func(t *testing.T) {
		_, err := manifest.Read(t.TempDir())

		if !errors.Is(err, domain.ErrNoManifest) || !strings.Contains(err.Error(), "lint-setup") || !strings.Contains(err.Error(), domain.ManifestFile) {
			t.Fatalf("Read = %v, want ErrNoManifest naming the file and the lint-setup skill", err)
		}
	})

	t.Run("a file that is not YAML", func(t *testing.T) {
		_, err := manifest.Read(writeManifest(t, "pack: [ddd\n"))

		if err == nil || !strings.Contains(err.Error(), domain.ManifestFile) {
			t.Fatalf("Read = %v, want a decode error naming %s", err, domain.ManifestFile)
		}
	})
}
