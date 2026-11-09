package manifest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/90p-labs/harness/internal/lint/domain"
)

func Read(proof string) (domain.Manifest, error) {
	path := filepath.Join(proof, domain.ManifestFile)
	file, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return domain.Manifest{}, fmt.Errorf("%w: %s", domain.ErrNoManifest, path)
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
