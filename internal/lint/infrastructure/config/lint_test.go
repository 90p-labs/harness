package config_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/lint/domain"
	lintconfig "github.com/jorge07/harness/internal/lint/infrastructure/config"
	sharedconfig "github.com/jorge07/harness/internal/shared/infrastructure/config"
)

func TestDecodeLint(t *testing.T) {
	t.Run("good config", func(t *testing.T) {
		packs, settings, err := lintconfig.Decode(sharedconfig.Lint{
			Packs:         []string{"ddd", "ddd-es", "nextjs", "nextjs-ddd-shadcn", "observability", "shadcn"},
			DDD:           sharedconfig.LintDDD{Root: "contexts", DomainAllow: []string{"zod"}, SrcFiles: []string{"main.ts"}},
			Shadcn:        sharedconfig.LintShadcn{UIAllow: []string{"clsx"}},
			Observability: sharedconfig.LintObservability{Chart: "deploy/chart", Rules: "deploy/**/*.rules.yaml", Dashboards: "deploy/**/dashboards/*.json"},
		})

		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		wantPacks := []domain.PackName{
			domain.PackDDD, domain.PackDDDES, domain.PackNextjs,
			domain.PackNextjsDDDShadcn, domain.PackObservability, domain.PackShadcn,
		}
		if !reflect.DeepEqual(packs, wantPacks) {
			t.Errorf("packs = %v, want %v", packs, wantPacks)
		}
		wantSettings := domain.Settings{
			DDD:           domain.DDD{Root: "contexts", DomainAllow: []string{"zod"}, SrcFiles: []string{"main.ts"}},
			Shadcn:        domain.Shadcn{UIAllow: []string{"clsx"}},
			Observability: domain.Observability{Chart: "deploy/chart", Rules: "deploy/**/*.rules.yaml", Dashboards: "deploy/**/dashboards/*.json"},
		}
		if !reflect.DeepEqual(settings, wantSettings) {
			t.Errorf("settings = %+v, want %+v", settings, wantSettings)
		}
	})

	t.Run("no pack", func(t *testing.T) {
		packs, _, err := lintconfig.Decode(sharedconfig.Lint{})
		if err != nil || len(packs) != 0 {
			t.Fatalf("decode = %v, %v, want no pack and no error", packs, err)
		}
	})

	t.Run("a repeated pack counts once", func(t *testing.T) {
		packs, _, err := lintconfig.Decode(sharedconfig.Lint{Packs: []string{"ddd", "shadcn", "ddd"}})

		if want := []domain.PackName{domain.PackDDD, domain.PackShadcn}; err != nil || !reflect.DeepEqual(packs, want) {
			t.Fatalf("decode = %v, %v, want %v", packs, err, want)
		}
	})

	t.Run("unknown pack", func(t *testing.T) {
		_, _, err := lintconfig.Decode(sharedconfig.Lint{Packs: []string{"ddd", "django"}})

		if !errors.Is(err, domain.ErrUnknownPack) || !strings.Contains(err.Error(), "django") {
			t.Fatalf("decode = %v, want ErrUnknownPack naming django", err)
		}
	})
}
