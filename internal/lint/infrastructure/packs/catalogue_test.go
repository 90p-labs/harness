package packs_test

import (
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/packs"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

func TestPackCatalogue(t *testing.T) {
	all, err := packs.All()
	if err != nil {
		t.Fatalf("All: %v", err)
	}

	wantRules := map[domain.PackName]int{
		domain.PackDDD:             23,
		domain.PackDDDES:           3,
		domain.PackNextjs:          7,
		domain.PackNextjsDDDShadcn: 3,
		domain.PackObservability:   10,
		domain.PackShadcn:          4,
	}
	if len(all) != len(wantRules) {
		t.Fatalf("packs = %d, want %d", len(all), len(wantRules))
	}
	ids := map[string]domain.PackName{}
	grit := 0
	for _, pack := range all {
		if got, want := len(pack.Rules), wantRules[pack.Name]; got != want {
			t.Errorf("pack %s has %d rules, want %d", pack.Name, got, want)
		}
		if _, err := fs.Stat(packs.Files, path.Join(string(pack.Name), "README.md")); err != nil {
			t.Errorf("pack %s: %v", pack.Name, err)
		}
		for _, rule := range pack.Rules {
			if owner, dup := ids[rule.ID]; dup {
				t.Errorf("rule %s is in packs %s and %s", rule.ID, owner, pack.Name)
			}
			ids[rule.ID] = pack.Name
			if !slices.Contains([]shareddomain.Severity{shareddomain.SeverityError, shareddomain.SeverityWarn, shareddomain.SeverityInfo}, rule.Severity) {
				t.Errorf("rule %s has severity %q", rule.ID, rule.Severity)
			}
			if rule.Scope == "" || rule.Message == "" {
				t.Errorf("rule %s has no scope or no message: %+v", rule.ID, rule)
			}
			switch rule.Engine {
			case domain.EngineBiomeGrit:
				grit++
				source, err := fs.ReadFile(packs.Files, rule.Plugin)
				if err != nil {
					t.Errorf("rule %s: plugin file: %v", rule.ID, err)
				}
				if !strings.Contains(string(source), "["+rule.ID+"]") {
					t.Errorf("rule %s: the plugin message does not start with the rule id", rule.ID)
				}
			case domain.EngineBiomeRule:
				if !strings.HasPrefix(rule.BiomeRule, "lint/") {
					t.Errorf("rule %s has biome category %q", rule.ID, rule.BiomeRule)
				}
			case domain.EngineOxlint:
				if rule.OxlintRule == "" || rule.Reason == "" {
					t.Errorf("rule %s has no oxlint rule or no reason", rule.ID)
				}
			case domain.EngineHarness:
			default:
				t.Errorf("rule %s has engine %q", rule.ID, rule.Engine)
			}
		}
	}
	if grit != 10 {
		t.Errorf("biome-grit rules = %d, want 10", grit)
	}

	wantRequires := map[domain.PackName][]domain.PackName{
		domain.PackDDDES:           {domain.PackDDD},
		domain.PackNextjsDDDShadcn: {domain.PackDDD, domain.PackNextjs, domain.PackShadcn},
	}
	for _, pack := range all {
		if !slices.Equal(pack.Requires, wantRequires[pack.Name]) {
			t.Errorf("pack %s requires %v, want %v", pack.Name, pack.Requires, wantRequires[pack.Name])
		}
	}
}

func TestPackByName(t *testing.T) {
	got, err := packs.Get(domain.PackShadcn)
	if err != nil || got.Name != domain.PackShadcn || len(got.Rules) != 4 {
		t.Fatalf("Get = %+v, %v", got, err)
	}
	if _, err := packs.Get("nope"); !errors.Is(err, domain.ErrUnknownPack) {
		t.Fatalf("Get unknown = %v, want ErrUnknownPack", err)
	}
}
