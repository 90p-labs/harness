package manifest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/jorge07/harness/internal/lint/domain"
)

func Read(repo string) (domain.Manifest, error) {
	file, err := os.Open(filepath.Join(repo, filepath.FromSlash(domain.ManifestFile)))
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Manifest{}, domain.ErrNoManifest
	}
	if err != nil {
		return domain.Manifest{}, err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var manifest domain.Manifest
	if err := decoder.Decode(&manifest); err != nil && !errors.Is(err, io.EOF) {
		return domain.Manifest{}, fmt.Errorf("decode %s: %w", domain.ManifestFile, err)
	}
	return manifest, nil
}
