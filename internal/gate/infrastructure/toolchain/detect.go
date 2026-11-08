package toolchain

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

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

	idLint         = "lint"
	idHelmRender   = "helm-render"
	idPromtoolTest = "promtool-test"

	runLint         = "harness lint --json"
	runHelmRender   = "helm template %s > /dev/null"
	runPromtoolTest = "find %s -name '*.rules.test.yaml' -exec promtool test rules {} +"

	packObservability = "observability"
	currentDir        = "."
)

var (
	goSources   = []string{"**/*.go"}
	nodeScripts = []string{"typecheck", "lint", "test"}
)

type Lint struct {
	Packs    []string
	Chart    string
	Rules    string
	Baseline string
}

func (l Lint) gates() ([]domain.Gate, error) {
	if len(l.Packs) == 0 {
		return nil, nil
	}
	lint, err := domain.NewGate(idLint, runLint, nil, 0, l.Baseline)
	if err != nil {
		return nil, err
	}
	gates := []domain.Gate{lint}
	if !slices.Contains(l.Packs, packObservability) {
		return gates, nil
	}
	observability, err := newGates(
		gateSpec{idHelmRender, fmt.Sprintf(runHelmRender, l.Chart), nil},
		gateSpec{idPromtoolTest, fmt.Sprintf(runPromtoolTest, literalDir(l.Rules)), nil},
	)
	return append(gates, observability...), err
}

func literalDir(glob string) string {
	var dir []string
	for _, segment := range strings.Split(glob, "/") {
		if strings.Contains(segment, "*") {
			break
		}
		dir = append(dir, segment)
	}
	return cmp.Or(path.Join(dir...), currentDir)
}

func Detect(dir string, lint Lint) ([]domain.Gate, error) {
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
	lintGates, err := lint.gates()
	if err != nil {
		return nil, err
	}
	return append(gates, lintGates...), nil
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
