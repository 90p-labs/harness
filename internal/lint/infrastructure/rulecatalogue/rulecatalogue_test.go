package rulecatalogue_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/packs"
	"github.com/90p-labs/harness/internal/lint/infrastructure/rulecatalogue"
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
	want := []string{"ddd/src-roots", "ddd/domain-purity", "ddd/layer-direction", "ddd/cross-context", "ddd/file-suffix"}
	if !slices.Equal(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestCatalogueSrcRoots(t *testing.T) {
	catalogue, err := rulecatalogue.Load()
	if err != nil {
		t.Fatal(err)
	}
	pack, err := catalogue.Pack(domain.PackDDD)
	if err != nil {
		t.Fatal(err)
	}
	at := slices.IndexFunc(pack.Rules, func(r domain.CatalogueRule) bool { return r.ID == domain.RuleSrcRoots })
	if at < 0 {
		t.Fatalf("the catalogue has no %s rule", domain.RuleSrcRoots)
	}
	for _, word := range []string{"src/", "TypeScript", "ddd.src_files"} {
		if strings.Contains(pack.Rules[at].Description, word) {
			t.Errorf("%s description names %q, want words for any language", domain.RuleSrcRoots, word)
		}
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
