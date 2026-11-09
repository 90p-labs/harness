package config_test

import (
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/lint/domain"
	lintconfig "github.com/90p-labs/harness/internal/lint/infrastructure/config"
	sharedconfig "github.com/90p-labs/harness/internal/shared/infrastructure/config"
)

func TestDecodeLint(t *testing.T) {
	settings := lintconfig.Decode(sharedconfig.Lint{
		Packs:         []string{"ddd", "./lint/team"},
		DDD:           sharedconfig.LintDDD{Root: "contexts", DomainAllow: []string{"zod"}, SrcFiles: []string{"main.ts"}},
		Shadcn:        sharedconfig.LintShadcn{UIAllow: []string{"clsx"}},
		Observability: sharedconfig.LintObservability{Chart: "deploy/chart", Rules: "deploy/**/*.rules.yaml", Dashboards: "deploy/**/dashboards/*.json"},
	})

	want := domain.Settings{
		DDD:           domain.DDD{Root: "contexts", DomainAllow: []string{"zod"}, SrcFiles: []string{"main.ts"}},
		Shadcn:        domain.Shadcn{UIAllow: []string{"clsx"}},
		Observability: domain.Observability{Chart: "deploy/chart", Rules: "deploy/**/*.rules.yaml", Dashboards: "deploy/**/dashboards/*.json"},
	}
	if !reflect.DeepEqual(settings, want) {
		t.Errorf("settings = %+v, want %+v", settings, want)
	}
}
