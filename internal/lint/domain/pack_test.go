package domain_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
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

func TestNewCustomPack(t *testing.T) {
	grit := domain.Rule{ID: "team/no-console", Engine: domain.EngineBiomeGrit, Severity: shareddomain.SeverityError, Plugin: "team/rules/no-console.grit"}
	cases := []struct {
		name     string
		pack     string
		requires []domain.PackName
		rule     domain.Rule
		want     error
	}{
		{name: "a valid pack", pack: "team", requires: []domain.PackName{domain.PackDDD}, rule: grit},
		{name: "a lint plugin engine", pack: "team", rule: domain.Rule{ID: "team/no-todo", Engine: "todo-lint", Severity: shareddomain.SeverityWarn}},
		{name: "an unknown severity", pack: "team", rule: domain.Rule{ID: "team/no-todo", Engine: "todo-lint", Severity: "warning"}, want: domain.ErrUnknownSeverity},
		{name: "no severity", pack: "team", rule: domain.Rule{ID: "team/no-todo", Engine: "todo-lint"}, want: domain.ErrUnknownSeverity},
		{name: "a built-in name", pack: string(domain.PackDDD), rule: grit, want: domain.ErrPackNameTaken},
		{name: "an empty name", pack: "", rule: grit, want: domain.ErrInvalidPackName},
		{name: "a parent name", pack: "..", rule: grit, want: domain.ErrInvalidPackName},
		{name: "a name with a slash", pack: "team/a", rule: grit, want: domain.ErrInvalidPackName},
		{name: "an upper case name", pack: "Team", rule: grit, want: domain.ErrInvalidPackName},
		{name: "a custom requires", pack: "team", requires: []domain.PackName{"other"}, rule: grit, want: domain.ErrUnknownPack},
		{name: "a harness rule", pack: "team", rule: domain.Rule{ID: "team/layout", Engine: domain.EngineHarness}, want: domain.ErrCustomPackEngine},
		{name: "an empty engine", pack: "team", rule: domain.Rule{ID: "team/x"}, want: domain.ErrCustomPackEngine},
		{name: "a plugin outside the pack", pack: "team", rule: domain.Rule{ID: "team/x", Engine: domain.EngineBiomeGrit, Plugin: "ddd/rules/event-readonly.grit"}, want: domain.ErrPluginOutsidePack},
		{name: "a plugin with a parent element", pack: "team", rule: domain.Rule{ID: "team/x", Engine: domain.EngineBiomeGrit, Plugin: "team/../ddd/x.grit"}, want: domain.ErrPluginOutsidePack},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pack, err := domain.NewCustomPack(tc.pack, tc.requires, []domain.Rule{tc.rule})

			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("NewCustomPack = %v, want %v", err, tc.want)
				}
				return
			}
			want := domain.Pack{Name: domain.PackName(tc.pack), Requires: tc.requires, Rules: []domain.Rule{tc.rule}}
			if err != nil || !reflect.DeepEqual(pack, want) {
				t.Fatalf("NewCustomPack = %+v, %v, want %+v", pack, err, want)
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
