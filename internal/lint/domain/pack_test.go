package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
)

func TestPackRequires(t *testing.T) {
	cases := []struct {
		name    string
		pack    domain.Pack
		active  []domain.PackName
		missing domain.PackName
	}{
		{
			name:   "ddd needs nothing",
			pack:   domain.Pack{Name: domain.PackDDD},
			active: []domain.PackName{domain.PackDDD},
		},
		{
			name:    "ddd-es without ddd",
			pack:    domain.Pack{Name: domain.PackDDDES, Requires: []domain.PackName{domain.PackDDD}},
			active:  []domain.PackName{domain.PackDDDES, domain.PackNextjs},
			missing: domain.PackDDD,
		},
		{
			name:   "ddd-es with ddd",
			pack:   domain.Pack{Name: domain.PackDDDES, Requires: []domain.PackName{domain.PackDDD}},
			active: []domain.PackName{domain.PackDDDES, domain.PackDDD},
		},
		{
			name: "combo without shadcn",
			pack: domain.Pack{
				Name:     domain.PackNextjsDDDShadcn,
				Requires: []domain.PackName{domain.PackDDD, domain.PackNextjs, domain.PackShadcn},
			},
			active:  []domain.PackName{domain.PackNextjsDDDShadcn, domain.PackDDD, domain.PackNextjs},
			missing: domain.PackShadcn,
		},
		{
			name: "combo with its three packs",
			pack: domain.Pack{
				Name:     domain.PackNextjsDDDShadcn,
				Requires: []domain.PackName{domain.PackDDD, domain.PackNextjs, domain.PackShadcn},
			},
			active: []domain.PackName{domain.PackNextjsDDDShadcn, domain.PackShadcn, domain.PackDDD, domain.PackNextjs},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.pack.CheckRequires(tc.active)
			if tc.missing == "" {
				if err != nil {
					t.Fatalf("CheckRequires = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, domain.ErrMissingPack) {
				t.Fatalf("CheckRequires = %v, want ErrMissingPack", err)
			}
			for _, name := range []domain.PackName{tc.pack.Name, tc.missing} {
				if !strings.Contains(err.Error(), string(name)) {
					t.Errorf("error %q does not name %q", err, name)
				}
			}
		})
	}
}

func TestRulesByEngine(t *testing.T) {
	grit := domain.Rule{ID: "ddd/grit", Engine: domain.EngineBiomeGrit}
	builtin := domain.Rule{ID: "next/biome", Engine: domain.EngineBiomeRule}
	layout := domain.Rule{ID: "ddd/layout", Engine: domain.EngineHarness}
	packs := map[domain.PackName]domain.Pack{
		domain.PackDDD:   {Name: domain.PackDDD, Rules: []domain.Rule{grit, layout}},
		domain.PackDDDES: {Name: domain.PackDDDES, Requires: []domain.PackName{domain.PackDDD}, Rules: []domain.Rule{builtin}},
	}
	lookup := func(name domain.PackName) (domain.Pack, error) { return packs[name], nil }

	got, err := domain.RulesByEngine([]domain.PackName{domain.PackDDD, domain.PackDDDES}, lookup)

	if err != nil {
		t.Fatal(err)
	}
	want := map[domain.EngineName][]domain.Rule{
		domain.EngineBiomeGrit: {grit, builtin},
		domain.EngineHarness:   {layout},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RulesByEngine = %+v, want %+v", got, want)
	}
	if _, err := domain.RulesByEngine([]domain.PackName{domain.PackDDDES}, lookup); !errors.Is(err, domain.ErrMissingPack) {
		t.Fatalf("err = %v, want ErrMissingPack", err)
	}
}
