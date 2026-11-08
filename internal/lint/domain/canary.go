package domain

import (
	"fmt"
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
	ID     string  `yaml:"id"`
	Canary string  `yaml:"canary"`
	Waived *string `yaml:"waived"`
}

type CanaryResult struct {
	Rule   string
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
	for _, rule := range m.Rules {
		if err := rule.check(known, listed); err != nil {
			return nil, err
		}
		listed[rule.ID] = rule
	}
	if len(m.Rules) > 0 && !slices.ContainsFunc(m.Rules, func(r ManifestRule) bool { return r.Canary != "" }) {
		return nil, fmt.Errorf("%w in %s", ErrAllWaived, ManifestFile)
	}
	results := make([]CanaryResult, len(pack.Rules))
	for i, rule := range pack.Rules {
		entry, ok := listed[rule.ID]
		switch {
		case !ok:
			results[i] = CanaryResult{Rule: rule.ID, Status: CanaryFail, Detail: notListed}
		case entry.Waived != nil:
			results[i] = CanaryResult{Rule: rule.ID, Status: CanaryWaived, Detail: *entry.Waived}
		default:
			results[i] = CanaryResult{Rule: rule.ID, Canary: entry.Canary, Status: CanaryToRun}
		}
	}
	return results, nil
}

func (r ManifestRule) check(known map[string]bool, listed map[string]ManifestRule) error {
	_, twice := listed[r.ID]
	switch {
	case !known[r.ID]:
		return fmt.Errorf("%w in %s: %s", ErrUnknownRule, ManifestFile, r.ID)
	case twice:
		return fmt.Errorf("%w in %s: %s", ErrDuplicateRule, ManifestFile, r.ID)
	case (r.Canary == "") == (r.Waived == nil):
		return fmt.Errorf("%w in %s: %s", ErrCanaryOrWaiver, ManifestFile, r.ID)
	case r.Waived != nil && len(strings.Fields(*r.Waived)) < minWaiverWords:
		return fmt.Errorf("%w in %s: %s", ErrShortWaiver, ManifestFile, r.ID)
	case r.Canary != "" && !insideRepo(r.Canary):
		return fmt.Errorf("%w in %s: %s: %s", ErrCanaryOutsideRepo, ManifestFile, r.ID, r.Canary)
	}
	return nil
}

func insideRepo(folder string) bool {
	clean := path.Clean(folder)
	return !path.IsAbs(clean) && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}
