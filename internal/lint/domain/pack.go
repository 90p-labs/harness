package domain

import (
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

type PackName string

const (
	PackDDD             PackName = "ddd"
	PackDDDES           PackName = "ddd-es"
	PackNextjs          PackName = "nextjs"
	PackNextjsDDDShadcn PackName = "nextjs-ddd-shadcn"
	PackObservability   PackName = "observability"
	PackShadcn          PackName = "shadcn"
)

var AllPacks = []PackName{PackDDD, PackDDDES, PackNextjs, PackNextjsDDDShadcn, PackObservability, PackShadcn}

var packNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

func ParsePackName(name string) (PackName, error) {
	if !slices.Contains(AllPacks, PackName(name)) {
		return "", fmt.Errorf("%w: %q", ErrUnknownPack, name)
	}
	return PackName(name), nil
}

func NewCustomPack(name string, requires []PackName, rules []Rule) (Pack, error) {
	if !packNamePattern.MatchString(name) {
		return Pack{}, fmt.Errorf("%w: %q must match %s", ErrInvalidPackName, name, packNamePattern)
	}
	if slices.Contains(AllPacks, PackName(name)) {
		return Pack{}, fmt.Errorf("%w: %q is a built-in pack", ErrPackNameTaken, name)
	}
	for _, required := range requires {
		if _, err := ParsePackName(string(required)); err != nil {
			return Pack{}, fmt.Errorf("pack %s requires: %w", name, err)
		}
	}
	for _, rule := range rules {
		if rule.Engine == "" || rule.Engine == EngineHarness {
			return Pack{}, fmt.Errorf("%w: rule %s has %q", ErrCustomPackEngine, rule.ID, rule.Engine)
		}
		if rule.Plugin != "" && (!fs.ValidPath(rule.Plugin) || !strings.HasPrefix(rule.Plugin, name+"/")) {
			return Pack{}, fmt.Errorf("%w: rule %s has %q", ErrPluginOutsidePack, rule.ID, rule.Plugin)
		}
		if !rule.Severity.Known() {
			return Pack{}, fmt.Errorf("%w: rule %s has %q", ErrUnknownSeverity, rule.ID, rule.Severity)
		}
	}
	return Pack{Name: PackName(name), Requires: requires, Rules: rules}, nil
}

type Pack struct {
	Name     PackName
	Requires []PackName
	Rules    []Rule
}

func (p Pack) CheckRequires(active []PackName) error {
	for _, required := range p.Requires {
		if !slices.Contains(active, required) {
			return fmt.Errorf("%w: pack %s needs pack %s", ErrMissingPack, p.Name, required)
		}
	}
	return nil
}

func RulesByEngine(active []PackName, lookup func(PackName) (Pack, error)) (map[EngineName][]Rule, error) {
	byEngine := map[EngineName][]Rule{}
	for _, name := range active {
		pack, err := lookup(name)
		if err != nil {
			return nil, err
		}
		if err := pack.CheckRequires(active); err != nil {
			return nil, err
		}
		for _, rule := range pack.Rules {
			engine := rule.Engine
			// Biome runs the biome-rule and the biome-grit rules of a scope from one config.
			if engine == EngineBiomeRule {
				engine = EngineBiomeGrit
			}
			byEngine[engine] = append(byEngine[engine], rule)
		}
	}
	return byEngine, nil
}
