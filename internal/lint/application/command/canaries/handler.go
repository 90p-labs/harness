package canaries

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	noFindingFormat = "the engine %s gave no %s finding"
	outsideFormat   = "the engine %s gave a %s finding for %s, which is outside the canary"
	builtInHint     = ", and the built-in engine reads only TypeScript and JavaScript"
)

type Command struct {
	Dir string
}

type ReadManifest func(dir string) (domain.Manifest, error)

type CheckVersion func(dir string) error

type Handler struct {
	catalogue     domain.RuleCatalogue
	read          ReadManifest
	active        map[string]activeRule
	engines       domain.EngineSet
	fresh         CheckVersion
	needsManifest bool
}

type activeRule struct {
	rule   domain.Rule
	engine domain.EngineName
}

func NewHandler(catalogue domain.RuleCatalogue, read ReadManifest, rules map[domain.EngineName][]domain.Rule, engines domain.EngineSet, fresh CheckVersion) Handler {
	active := map[string]activeRule{}
	for engine, list := range rules {
		for _, rule := range list {
			active[rule.ID] = activeRule{rule: rule, engine: engine}
		}
	}
	needsManifest := len(catalogue.PluginRules(rules)) > 0
	return Handler{catalogue: catalogue, read: read, active: active, engines: engines, fresh: fresh, needsManifest: needsManifest}
}

func (h Handler) Handle(ctx context.Context, cmd Command) ([]domain.CanaryResult, error) {
	manifest, err := h.read(cmd.Dir)
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
	for _, result := range results {
		if result.Status != domain.CanaryToRun {
			continue
		}
		if _, ok := h.active[result.Rule]; !ok {
			return nil, fmt.Errorf("%w: %s", domain.ErrCanaryRuleInactive, result.Rule)
		}
		info, err := os.Stat(canaryDir(cmd.Dir, result.Canary))
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
		if results[i], err = h.run(ctx, cmd.Dir, result); err != nil {
			return nil, err
		}
	}
	return results, nil
}

func (h Handler) run(ctx context.Context, repo string, result domain.CanaryResult) (domain.CanaryResult, error) {
	active := h.active[result.Rule]
	canary := canaryDir(repo, result.Canary)
	findings, err := h.engines.Check(ctx, canary, active.engine, []domain.Rule{active.rule})
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
		result.Detail = fmt.Sprintf(outsideFormat, active.engine, result.Rule, outside)
	case active.engine == domain.EngineHarness:
		result.Detail = fmt.Sprintf(noFindingFormat, active.engine, result.Rule) + builtInHint
	default:
		result.Detail = fmt.Sprintf(noFindingFormat, active.engine, result.Rule)
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

func canaryDir(repo, canary string) string {
	return filepath.Join(repo, filepath.FromSlash(canary))
}
