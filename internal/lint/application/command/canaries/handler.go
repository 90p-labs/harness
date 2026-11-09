package canaries

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	noFindingFormat = "the engine %s gave no %s finding"
	outsideFormat   = "the engine %s gave a %s finding for %s, which is outside the canary"
	builtInHint     = ", and the built-in engine reads only TypeScript and JavaScript"
)

type Command struct {
	Dir   string
	Proof string
}

type ReadManifest func(dir string) (domain.Manifest, error)

type CheckVersion func(dir string) error

type Handler struct {
	catalogue     domain.RuleCatalogue
	read          ReadManifest
	active        map[string]map[domain.EngineName]domain.Rule
	engines       domain.EngineSet
	fresh         CheckVersion
	needsManifest bool
}

func NewHandler(catalogue domain.RuleCatalogue, read ReadManifest, rules map[domain.EngineName][]domain.Rule, engines domain.EngineSet, fresh CheckVersion) Handler {
	active := map[string]map[domain.EngineName]domain.Rule{}
	for engine, list := range rules {
		for _, rule := range list {
			if active[rule.ID] == nil {
				active[rule.ID] = map[domain.EngineName]domain.Rule{}
			}
			active[rule.ID][engine] = rule
		}
	}
	needsManifest := len(catalogue.PluginRules(rules)) > 0
	return Handler{catalogue: catalogue, read: read, active: active, engines: engines, fresh: fresh, needsManifest: needsManifest}
}

func (h Handler) Handle(ctx context.Context, cmd Command) ([]domain.CanaryResult, error) {
	proof, err := proofDir(cmd.Dir, cmd.Proof)
	if err != nil {
		return nil, err
	}
	manifest, err := h.read(proof)
	if errors.Is(err, domain.ErrNoManifest) && len(h.active) > 0 && !h.needsManifest {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if len(h.active) > 0 {
		if err := h.fresh(cmd.Dir); err != nil {
			return nil, err
		}
	}
	results, err := manifest.Results(h.catalogue)
	if err != nil {
		return nil, err
	}
	engines := map[string][]domain.EngineName{}
	for id, byEngine := range h.active {
		engines[id] = slices.Sorted(maps.Keys(byEngine))
	}
	if results, err = domain.ForEngines(results, engines); err != nil {
		return nil, err
	}
	for _, result := range results {
		if result.Status != domain.CanaryToRun {
			continue
		}
		info, err := os.Stat(canaryDir(proof, result.Canary))
		if err != nil {
			return nil, fmt.Errorf("%w: %s for %s", domain.ErrCanaryMissing, result.Canary, result.Rule)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%w: %s for %s", domain.ErrCanaryNotFolder, result.Canary, result.Rule)
		}
	}
	for i, result := range results {
		if result.Status != domain.CanaryToRun {
			continue
		}
		if results[i], err = h.run(ctx, proof, result, engines[result.Rule]); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (h Handler) run(ctx context.Context, proof string, result domain.CanaryResult, engines []domain.EngineName) (domain.CanaryResult, error) {
	engine := cmp.Or(result.Engine, engines[0])
	canary := canaryDir(proof, result.Canary)
	findings, err := h.engines.Check(ctx, canary, engine, []domain.Rule{h.active[result.Rule][engine]})
	if err != nil {
		return result, fmt.Errorf("canary %s for %s: %w", result.Canary, result.Rule, err)
	}
	outside := ""
	for _, f := range findings {
		if f.Rule != result.Rule {
			continue
		}
		if inside(canary, f) {
			result.Status = domain.CanaryPass
			return result, nil
		}
		outside = f.File
	}
	result.Status = domain.CanaryFail
	switch {
	case outside != "":
		result.Detail = fmt.Sprintf(outsideFormat, engine, result.Rule, outside)
	case engine == domain.EngineHarness:
		result.Detail = fmt.Sprintf(noFindingFormat, engine, result.Rule) + builtInHint
	default:
		result.Detail = fmt.Sprintf(noFindingFormat, engine, result.Rule)
	}
	return result, nil
}

func inside(canary string, f shareddomain.Finding) bool {
	file := filepath.FromSlash(f.File)
	if !filepath.IsLocal(file) {
		return false
	}
	_, err := os.Stat(filepath.Join(canary, file))
	return err == nil
}

func canaryDir(proof, canary string) string {
	return filepath.Join(proof, filepath.FromSlash(canary))
}

func proofDir(repo, proof string) (string, error) {
	dir := proof
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(repo, proof)
	}
	realRepo, realDir := repo, dir
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		if realRepo, err = filepath.EvalSymlinks(repo); err != nil {
			return "", err
		}
		realDir = resolved
	}
	if rel, err := filepath.Rel(realRepo, realDir); err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%w: %s", domain.ErrProofOutsideRepo, proof)
	}
	return dir, nil
}
