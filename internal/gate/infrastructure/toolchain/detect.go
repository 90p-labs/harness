package toolchain

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/jorge07/harness/internal/gate/domain"
)

const (
	fileGoMod       = "go.mod"
	filePackageJSON = "package.json"
	filePnpmLock    = "pnpm-lock.yaml"
	fileNpmLock     = "package-lock.json"
	fileBiomeJSON   = "biome.json"
	fileBiomeJSONC  = "biome.jsonc"

	idGoFmt  = "go-fmt"
	idGoVet  = "go-vet"
	idGoTest = "go-test"
	idBiome  = "biome"

	runGoFmt  = `test -z "$(gofmt -l .)"`
	runGoVet  = "go vet ./..."
	runGoTest = "go test ./..."

	runPnpmScript = "pnpm run "
	runNpmScript  = "npm run "
	runPnpmBiome  = "pnpm exec biome ci ."
	runNpxBiome   = "npx biome ci ."
)

var (
	goSources   = []string{"**/*.go"}
	nodeScripts = []string{"typecheck", "lint", "test"}
)

func Detect(dir string) ([]domain.Gate, error) {
	var gates []domain.Gate
	if exists(dir, fileGoMod) {
		goGates, err := newGates(
			gateSpec{idGoFmt, runGoFmt, goSources},
			gateSpec{idGoVet, runGoVet, nil},
			gateSpec{idGoTest, runGoTest, nil},
		)
		if err != nil {
			return nil, err
		}
		gates = append(gates, goGates...)
	}

	pnpm := exists(dir, filePnpmLock)
	if pnpm || exists(dir, fileNpmLock) {
		nodeGates, err := scriptGates(dir, pnpm)
		if err != nil {
			return nil, err
		}
		gates = append(gates, nodeGates...)
	}

	if exists(dir, fileBiomeJSON) || exists(dir, fileBiomeJSONC) {
		run := runNpxBiome
		if pnpm {
			run = runPnpmBiome
		}
		biome, err := newGates(gateSpec{idBiome, run, nil})
		if err != nil {
			return nil, err
		}
		gates = append(gates, biome...)
	}
	return gates, nil
}

type gateSpec struct {
	id    string
	run   string
	paths []string
}

func newGates(specs ...gateSpec) ([]domain.Gate, error) {
	gates := make([]domain.Gate, 0, len(specs))
	for _, spec := range specs {
		gate, err := domain.NewGate(spec.id, spec.run, spec.paths, 0, "")
		if err != nil {
			return nil, err
		}
		gates = append(gates, gate)
	}
	return gates, nil
}

func scriptGates(dir string, pnpm bool) ([]domain.Gate, error) {
	raw, err := os.ReadFile(filepath.Join(dir, filePackageJSON))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filePackageJSON, err)
	}
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePackageJSON, err)
	}
	runner := runNpmScript
	if pnpm {
		runner = runPnpmScript
	}
	var specs []gateSpec
	for _, name := range nodeScripts {
		if _, ok := manifest.Scripts[name]; ok {
			specs = append(specs, gateSpec{name, runner + name, nil})
		}
	}
	return newGates(specs...)
}

func exists(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}
