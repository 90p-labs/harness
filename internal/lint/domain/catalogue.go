package domain

type RuleCatalogue struct {
	Packs []CataloguePack `json:"packs"`
}

type CataloguePack struct {
	Name  PackName        `json:"name"`
	Rules []CatalogueRule `json:"rules"`
}

type CatalogueRule struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

// PluginRules are the active rules that a plugin engine checks under a catalogue id: only these need a canary.
func (c RuleCatalogue) PluginRules(rules map[EngineName][]Rule) []Rule {
	known := map[string]bool{}
	for _, pack := range c.Packs {
		for _, rule := range pack.Rules {
			known[rule.ID] = true
		}
	}
	var found []Rule
	for engine, list := range rules {
		for _, rule := range list {
			if engine.IsPlugin() && known[rule.ID] {
				found = append(found, rule)
			}
		}
	}
	return found
}
