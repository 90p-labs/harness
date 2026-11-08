package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/lint/domain"
)

func Read(repo string) (domain.Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(repo, filepath.FromSlash(domain.ManifestFile)))
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Manifest{}, domain.ErrNoManifest
	}
	if err != nil {
		return domain.Manifest{}, err
	}
	var manifest domain.Manifest
	if err := yaml.Unmarshal(raw, &manifest); err != nil {
		return domain.Manifest{}, fmt.Errorf("decode %s: %w", domain.ManifestFile, err)
	}
	return manifest, nil
}
