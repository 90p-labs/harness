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

func TestManifestRead(t *testing.T) {
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
	waiver := "deptrac checks dependencies, not file names."
	want := domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{
		{ID: "ddd/domain-purity", Canary: "lint/canaries/ddd-domain-purity"},
		{ID: "ddd/file-suffix", Waived: &waiver},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Read = %+v, want %+v", got, want)
	}
}

func TestManifestReadRefuses(t *testing.T) {
	t.Run("an unknown key", func(t *testing.T) {
		_, err := manifest.Read(writeManifest(t, "pack: ddd\nextra: 1\nrules: []\n"))

		if err == nil || !strings.Contains(err.Error(), "extra") || !strings.Contains(err.Error(), domain.ManifestFile) {
			t.Fatalf("Read = %v, want an error naming the unknown key extra", err)
		}
	})

	t.Run("an empty file reads as a manifest with no pack", func(t *testing.T) {
		got, err := manifest.Read(writeManifest(t, ""))

		if err != nil || !reflect.DeepEqual(got, domain.Manifest{}) {
			t.Fatalf("Read = %+v, %v, want an empty manifest, which Results refuses for its pack key", got, err)
		}
	})

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
