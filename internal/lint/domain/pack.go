package domain

import (
	"fmt"
	"slices"
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

func ParsePackName(name string) (PackName, error) {
	if !slices.Contains(AllPacks, PackName(name)) {
		return "", fmt.Errorf("%w: %q", ErrUnknownPack, name)
	}
	return PackName(name), nil
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
