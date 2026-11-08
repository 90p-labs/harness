package canaries

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

type Command struct {
	Dir string
}

type ReadManifest func(dir string) (domain.Manifest, error)

type Handler struct {
	catalogue domain.RuleCatalogue
	read      ReadManifest
	active    map[string]activeRule
	engines   domain.EngineSet
}

type activeRule struct {
	rule   domain.Rule
	engine domain.EngineName
}

func NewHandler(catalogue domain.RuleCatalogue, read ReadManifest, rules map[domain.EngineName][]domain.Rule, engines domain.EngineSet) Handler {
	active := map[string]activeRule{}
	for engine, list := range rules {
		for _, rule := range list {
			active[rule.ID] = activeRule{rule: rule, engine: engine}
		}
	}
	return Handler{catalogue: catalogue, read: read, active: active, engines: engines}
}

func (h Handler) Handle(ctx context.Context, cmd Command) ([]domain.CanaryResult, error) {
	manifest, err := h.read(cmd.Dir)
	if err != nil {
		return nil, err
	}
	results, err := manifest.Results(h.catalogue)
	if err != nil {
		return nil, err
	}
	for _, result := range results {
		if result.Status != domain.CanaryToRun {
			continue
		}
		if _, ok := h.active[result.Rule]; !ok {
			return nil, fmt.Errorf("%w: %s", domain.ErrCanaryRuleInactive, result.Rule)
		}
		if info, err := os.Stat(canaryDir(cmd.Dir, result.Canary)); err != nil || !info.IsDir() {
			return nil, fmt.Errorf("%w: %s for %s", domain.ErrCanaryMissing, result.Canary, result.Rule)
		}
	}
	for i, result := range results {
		if result.Status != domain.CanaryToRun {
			continue
		}
		if results[i], err = h.run(ctx, cmd.Dir, result); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (h Handler) run(ctx context.Context, repo string, result domain.CanaryResult) (domain.CanaryResult, error) {
	active := h.active[result.Rule]
	findings, err := h.engines.Check(ctx, canaryDir(repo, result.Canary), active.engine, []domain.Rule{active.rule})
	if err != nil {
		return result, fmt.Errorf("canary %s for %s: %w", result.Canary, result.Rule, err)
	}
	if slices.ContainsFunc(findings, func(f shareddomain.Finding) bool { return f.Rule == result.Rule }) {
		result.Status = domain.CanaryPass
		return result, nil
	}
	result.Status = domain.CanaryFail
	result.Detail = fmt.Sprintf("the engine %s gave no %s finding", active.engine, result.Rule)
	return result, nil
}

func canaryDir(repo, canary string) string {
	return filepath.Join(repo, filepath.FromSlash(canary))
}
