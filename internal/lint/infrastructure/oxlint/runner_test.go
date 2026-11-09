//go:build linters

package oxlint_test

import (
	"context"
	"path"
	"path/filepath"
	"testing"

	"github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/fixture"
	"github.com/90p-labs/harness/internal/lint/infrastructure/oxlint"
	"github.com/90p-labs/harness/internal/lint/infrastructure/packs"
)

const (
	archiveExtension = ".txtar"
	violationFile    = "src/app/violation.tsx"
	violationLine    = 3
)

func oxlintRules(t *testing.T) []domain.Rule {
	t.Helper()
	all, err := packs.All()
	if err != nil {
		t.Fatal(err)
	}
	var rules []domain.Rule
	for _, pack := range all {
		for _, rule := range pack.Rules {
			if rule.Engine == domain.EngineOxlint {
				rules = append(rules, rule)
			}
		}
	}
	if len(rules) == 0 {
		t.Fatal("the catalogue has no oxlint rule")
	}
	return rules
}

func TestOxlintEngineFixtures(t *testing.T) {
	engine := oxlint.New()
	if engine.Name() != domain.EngineOxlint {
		t.Fatalf("Name = %q, want %q", engine.Name(), domain.EngineOxlint)
	}
	for _, rule := range oxlintRules(t) {
		t.Run(rule.ID, func(t *testing.T) {
			if rule.Reason == "" {
				t.Errorf("rule %s has no reason", rule.ID)
			}
			dir := fixture.Extract(t, filepath.Join("testdata", "next", path.Base(rule.ID)+archiveExtension))
			rules := []domain.Rule{rule}
			if err := engine.Prepare(dir, rules); err != nil {
				t.Fatal(err)
			}
			findings, err := engine.Run(context.Background(), dir, rules)
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 {
				t.Fatalf("findings = %+v, want exactly one", findings)
			}
			got := findings[0]
			if got.Rule != rule.ID || got.File != violationFile || got.Line != violationLine || got.Severity != rule.Severity || got.Message != rule.Message {
				t.Errorf("finding = %+v, want rule %s in %s with the rule severity and message", got, rule.ID, violationFile)
			}
		})
	}
}
