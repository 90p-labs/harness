package rulecatalogue_test

import (
	"slices"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/packs"
	"github.com/jorge07/harness/internal/lint/infrastructure/rulecatalogue"
)

func TestLoad(t *testing.T) {
	catalogue, err := rulecatalogue.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(catalogue.Packs) != 1 || catalogue.Packs[0].Name != domain.PackDDD {
		t.Fatalf("packs = %+v, want only %s", catalogue.Packs, domain.PackDDD)
	}
	var ids []string
	for _, rule := range catalogue.Packs[0].Rules {
		if rule.Description == "" {
			t.Errorf("rule %s has no description", rule.ID)
		}
		ids = append(ids, rule.ID)
	}
	want := []string{"ddd/domain-purity", "ddd/layer-direction", "ddd/cross-context", "ddd/file-suffix"}
	if !slices.Equal(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestCatalogueIDsAreBuiltInIDs(t *testing.T) {
	catalogue, err := rulecatalogue.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, pack := range catalogue.Packs {
		builtin, err := packs.Get(pack.Name)
		if err != nil {
			t.Fatalf("catalogue pack %s: %v", pack.Name, err)
		}
		for _, rule := range pack.Rules {
			if !slices.ContainsFunc(builtin.Rules, func(r domain.Rule) bool { return r.ID == rule.ID }) {
				t.Errorf("catalogue rule %s is not a rule of the built-in pack %s", rule.ID, pack.Name)
			}
		}
	}
}
