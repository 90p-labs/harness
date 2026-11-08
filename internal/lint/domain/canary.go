package domain

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
)

const (
	ManifestFile    = "lint/rules.yaml"
	minWaiverWords  = 3
	manifestKeyPack = "pack"
	notListed       = "not in " + ManifestFile
)

type CanaryStatus string

const (
	CanaryToRun  CanaryStatus = "RUN"
	CanaryPass   CanaryStatus = "PASS"
	CanaryFail   CanaryStatus = "FAIL"
	CanaryWaived CanaryStatus = "WAIVED"
)

type Manifest struct {
	Pack  PackName       `yaml:"pack"`
	Rules []ManifestRule `yaml:"rules"`
}

type ManifestRule struct {
	ID      string                       `yaml:"id"`
	Canary  string                       `yaml:"canary"`
	Waived  *string                      `yaml:"waived"`
	Engines map[EngineName]ManifestProof `yaml:"engines"`
}

type ManifestProof struct {
	Canary string  `yaml:"canary"`
	Waived *string `yaml:"waived"`
}

type CanaryResult struct {
	Rule   string
	Engine EngineName
	Canary string
	Status CanaryStatus
	Detail string
}

func (c RuleCatalogue) Pack(name PackName) (CataloguePack, error) {
	for _, pack := range c.Packs {
		if pack.Name == name {
			return pack, nil
		}
	}
	return CataloguePack{}, fmt.Errorf("%w in %s: %s", ErrUnknownPack, ManifestFile, name)
}

func (m Manifest) Results(catalogue RuleCatalogue) ([]CanaryResult, error) {
	if m.Pack == "" {
		return nil, fmt.Errorf("%w in %s: set %s to a pack that harness lint catalogue lists", ErrNoPackKey, ManifestFile, manifestKeyPack)
	}
	pack, err := catalogue.Pack(m.Pack)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, rule := range pack.Rules {
		known[rule.ID] = true
	}
	listed := map[string]ManifestRule{}
	anyCanary := false
	for _, rule := range m.Rules {
		if err := rule.check(known, listed); err != nil {
			return nil, err
		}
		listed[rule.ID] = rule
		anyCanary = anyCanary || slices.ContainsFunc(rule.proofs(), func(p ManifestProof) bool { return p.Canary != "" })
	}
	if len(m.Rules) > 0 && !anyCanary {
		return nil, fmt.Errorf("%w in %s", ErrAllWaived, ManifestFile)
	}
	var results []CanaryResult
	for _, rule := range pack.Rules {
		entry, ok := listed[rule.ID]
		switch {
		case !ok:
			results = append(results, CanaryResult{Rule: rule.ID, Status: CanaryFail, Detail: notListed})
		case entry.Engines != nil:
			for _, engine := range slices.Sorted(maps.Keys(entry.Engines)) {
				results = append(results, entry.Engines[engine].result(rule.ID, engine))
			}
		default:
			results = append(results, entry.single().result(rule.ID, ""))
		}
	}
	return results, nil
}

func (r ManifestRule) single() ManifestProof {
	return ManifestProof{Canary: r.Canary, Waived: r.Waived}
}

func (r ManifestRule) proofs() []ManifestProof {
	if r.Engines == nil {
		return []ManifestProof{r.single()}
	}
	return slices.Collect(maps.Values(r.Engines))
}

func (p ManifestProof) result(rule string, engine EngineName) CanaryResult {
	if p.Waived != nil {
		return CanaryResult{Rule: rule, Engine: engine, Status: CanaryWaived, Detail: *p.Waived}
	}
	return CanaryResult{Rule: rule, Engine: engine, Canary: p.Canary, Status: CanaryToRun}
}

func (r ManifestRule) check(known map[string]bool, listed map[string]ManifestRule) error {
	_, twice := listed[r.ID]
	switch {
	case !known[r.ID]:
		return fmt.Errorf("%w in %s: %s", ErrUnknownRule, ManifestFile, r.ID)
	case twice:
		return fmt.Errorf("%w in %s: %s", ErrDuplicateRule, ManifestFile, r.ID)
	case r.Engines == nil:
		return r.single().check(r.ID)
	case len(r.Engines) == 0 || r.Canary != "" || r.Waived != nil:
		return fmt.Errorf("%w in %s: %s: give the canary and the waiver under engines only", ErrCanaryOrWaiver, ManifestFile, r.ID)
	}
	for _, engine := range slices.Sorted(maps.Keys(r.Engines)) {
		if err := r.Engines[engine].check(r.ID + " " + string(engine)); err != nil {
			return err
		}
	}
	return nil
}

func (p ManifestProof) check(name string) error {
	switch {
	case (p.Canary == "") == (p.Waived == nil):
		return fmt.Errorf("%w in %s: %s", ErrCanaryOrWaiver, ManifestFile, name)
	case p.Waived != nil && len(strings.Fields(*p.Waived)) < minWaiverWords:
		return fmt.Errorf("%w in %s: %s", ErrShortWaiver, ManifestFile, name)
	case p.Canary != "" && !insideRepo(p.Canary):
		return fmt.Errorf("%w in %s: %s: %s", ErrCanaryOutsideRepo, ManifestFile, name, p.Canary)
	}
	return nil
}

// ForEngines matches each result to the engines that hold its rule. active lists them sorted.
func ForEngines(results []CanaryResult, active map[string][]EngineName) ([]CanaryResult, error) {
	var matched []CanaryResult
	for start := 0; start < len(results); {
		rule := results[start].Rule
		end := start + 1
		for end < len(results) && results[end].Rule == rule {
			end++
		}
		group, err := forRuleEngines(results[start:end], active[rule])
		if err != nil {
			return nil, err
		}
		matched = append(matched, group...)
		start = end
	}
	return matched, nil
}

func forRuleEngines(group []CanaryResult, engines []EngineName) ([]CanaryResult, error) {
	first := group[0]
	if first.Engine == "" {
		switch {
		case len(engines) > 1 && first.Status == CanaryFail:
			var failed []CanaryResult
			for _, engine := range engines {
				failed = append(failed, CanaryResult{Rule: first.Rule, Engine: engine, Status: CanaryFail, Detail: notListed})
			}
			return failed, nil
		case len(engines) > 1 && first.Status != CanaryFail:
			names := make([]string, len(engines))
			for i, engine := range engines {
				names[i] = string(engine)
			}
			return nil, fmt.Errorf("%w in %s: %s runs on %s", ErrOneProofForEngines, ManifestFile, first.Rule, strings.Join(names, " and "))
		case len(engines) == 0 && first.Status == CanaryToRun:
			return nil, fmt.Errorf("%w: %s", ErrCanaryRuleInactive, first.Rule)
		}
		return group, nil
	}
	listed := map[EngineName]bool{}
	for _, result := range group {
		if !slices.Contains(engines, result.Engine) {
			return nil, fmt.Errorf("%w: %s on the engine %s", ErrCanaryRuleInactive, result.Rule, result.Engine)
		}
		listed[result.Engine] = true
	}
	matched := slices.Clone(group)
	for _, engine := range engines {
		if !listed[engine] {
			matched = append(matched, CanaryResult{Rule: first.Rule, Engine: engine, Status: CanaryFail, Detail: notListed})
		}
	}
	slices.SortStableFunc(matched, func(a, b CanaryResult) int { return strings.Compare(string(a.Engine), string(b.Engine)) })
	return matched, nil
}

func insideRepo(folder string) bool {
	clean := path.Clean(folder)
	return !path.IsAbs(clean) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}
