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
	proof := t.TempDir()
	if err := os.WriteFile(filepath.Join(proof, domain.ManifestFile), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return proof
}

func TestManifestRead(t *testing.T) {
	repo := writeManifest(t, `pack: ddd
rules:
  - id: ddd/domain-purity
    canary: ddd-domain-purity
  - id: ddd/file-suffix
    waived: deptrac checks dependencies, not file names.
`)

	got, err := manifest.Read(repo)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	waiver := "deptrac checks dependencies, not file names."
	want := domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{
		{ID: "ddd/domain-purity", Canary: "ddd-domain-purity"},
		{ID: "ddd/file-suffix", Waived: &waiver},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Read = %+v, want %+v", got, want)
	}
}

func TestManifestReadEngines(t *testing.T) {
	repo := writeManifest(t, `pack: ddd
rules:
  - id: ddd/domain-purity
    engines:
      harness:
        waived: the binary fixtures prove it.
      deptrac:
        canary: ddd-domain-purity
`)

	got, err := manifest.Read(repo)

	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	waiver := "the binary fixtures prove it."
	want := domain.Manifest{Pack: domain.PackDDD, Rules: []domain.ManifestRule{
		{ID: "ddd/domain-purity", Engines: map[domain.EngineName]domain.ManifestProof{
			domain.EngineHarness: {Waived: &waiver},
			"deptrac":            {Canary: "ddd-domain-purity"},
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Read = %+v, want %+v", got, want)
	}

	for name, content := range map[string]string{
		"an engine named twice":       "pack: ddd\nrules:\n  - id: ddd/domain-purity\n    engines:\n      deptrac:\n        canary: a\n      deptrac:\n        canary: b\n",
		"an unknown key of an engine": "pack: ddd\nrules:\n  - id: ddd/domain-purity\n    engines:\n      deptrac:\n        canary: a\n        extra: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := manifest.Read(writeManifest(t, content)); err == nil || !strings.Contains(err.Error(), domain.ManifestFile) {
				t.Fatalf("Read = %v, want a decode error naming %s", err, domain.ManifestFile)
			}
		})
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

	t.Run("no manifest names the path and the setup skill", func(t *testing.T) {
		proof := filepath.Join(t.TempDir(), "proof")

		_, err := manifest.Read(proof)

		if !errors.Is(err, domain.ErrNoManifest) || !strings.Contains(err.Error(), "lint-setup") || !strings.Contains(err.Error(), filepath.Join(proof, domain.ManifestFile)) || strings.Contains(err.Error(), "lint/rules.yaml") {
			t.Fatalf("Read = %v, want ErrNoManifest naming the path of the file and the lint-setup skill", err)
		}
	})

	t.Run("a file that is not YAML", func(t *testing.T) {
		_, err := manifest.Read(writeManifest(t, "pack: [ddd\n"))

		if err == nil || !strings.Contains(err.Error(), domain.ManifestFile) {
			t.Fatalf("Read = %v, want a decode error naming %s", err, domain.ManifestFile)
		}
	})
}
