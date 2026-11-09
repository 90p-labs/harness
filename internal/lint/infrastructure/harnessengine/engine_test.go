package harnessengine

import (
	"context"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/deploy/infrastructure/toolplugin"
	"github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/fixture"
	"github.com/90p-labs/harness/internal/lint/infrastructure/packs"
)

const (
	archiveGlob = "*.txtar"
	cleanFolder = "clean"
)

func harnessRulesOf(t *testing.T, name domain.PackName) []domain.Rule {
	t.Helper()
	all, err := packs.All()
	if err != nil {
		t.Fatal(err)
	}
	wanted := []domain.PackName{name}
	for _, pack := range all {
		if pack.Name == name {
			wanted = append(wanted, pack.Requires...)
		}
	}
	var rules []domain.Rule
	for _, pack := range all {
		if !slices.Contains(wanted, pack.Name) {
			continue
		}
		for _, rule := range pack.Rules {
			if rule.Engine == domain.EngineHarness {
				rules = append(rules, rule)
			}
		}
	}
	return rules
}

func newTestEngine() *Engine {
	return New(domain.Settings{
		DDD: domain.DDD{Root: "contexts", SrcFiles: []string{"main.ts"}},
		Observability: domain.Observability{
			Chart:      "deploy/chart",
			Rules:      "deploy/**/*.rules.yaml",
			Dashboards: "deploy/**/dashboards/*.json",
		},
	}, toolplugin.Template)
}

func TestHarnessEngineFixtures(t *testing.T) {
	all, err := packs.All()
	if err != nil {
		t.Fatal(err)
	}
	ownerOf := map[string]domain.Pack{}
	for _, pack := range all {
		for _, rule := range pack.Rules {
			if rule.Engine == domain.EngineHarness {
				ownerOf[rule.ID] = pack
			}
		}
	}

	archives, err := filepath.Glob(filepath.Join("testdata", "*", archiveGlob))
	if err != nil {
		t.Fatal(err)
	}
	covered := map[string]bool{}
	cleanPacks := map[domain.PackName]bool{}
	for _, archive := range archives {
		folder := filepath.Base(filepath.Dir(archive))
		name := strings.TrimSuffix(filepath.Base(archive), ".txtar")
		if folder == cleanFolder {
			pack := domain.PackName(name)
			cleanPacks[pack] = true
			t.Run(cleanFolder+"/"+name, func(t *testing.T) {
				findings, err := newTestEngine().Run(context.Background(), fixture.Extract(t, archive), harnessRulesOf(t, pack))
				if err != nil {
					t.Fatal(err)
				}
				if len(findings) != 0 {
					t.Errorf("findings = %+v, want none", findings)
				}
			})
			continue
		}
		id := folder + "/" + name
		owner, ok := ownerOf[id]
		if !ok {
			t.Errorf("archive %s names no harness rule", archive)
			continue
		}
		covered[id] = true
		t.Run(id, func(t *testing.T) {
			findings, err := newTestEngine().Run(context.Background(), fixture.Extract(t, archive), harnessRulesOf(t, owner.Name))
			if err != nil {
				t.Fatal(err)
			}
			if len(findings) != 1 {
				t.Fatalf("findings = %+v, want exactly one", findings)
			}
			got := findings[0]
			rule := ruleByID(owner, id)
			if got.Rule != id || got.Severity != rule.Severity || got.Message != rule.Message || got.File == "" || got.Line < 1 {
				t.Errorf("finding = %+v, want rule %s with severity %s and message %q", got, id, rule.Severity, rule.Message)
			}
		})
	}

	for id, owner := range ownerOf {
		_, isChartRule := chartChecks[id]
		if cleanPacks[owner.Name] && !isChartRule && !covered[id] {
			t.Errorf("rule %s has no fixture archive", id)
		}
	}
}

func TestObservabilityPathsFromSettings(t *testing.T) {
	dir := t.TempDir()
	alert := "groups:\n  - name: api\n    rules:\n      - alert: APIDown\n        expr: up == 0\n"
	if err := os.MkdirAll(filepath.Join(dir, "ops"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ops", "api.rules.yaml"), []byte(alert), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	var runbookRule []domain.Rule
	for _, rule := range harnessRulesOf(t, domain.PackObservability) {
		if rule.ID == domain.RuleRunbook {
			runbookRule = append(runbookRule, rule)
		}
	}
	engine := New(domain.Settings{Observability: domain.Observability{Chart: "ops/chart", Rules: "ops/*.rules.yaml", Dashboards: "ops/dashboards/*.json"}}, toolplugin.Template)

	findings, err := engine.Run(context.Background(), dir, runbookRule)

	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Rule != domain.RuleRunbook || findings[0].File != "ops/api.rules.yaml" {
		t.Fatalf("findings = %+v, want one obs/runbook finding on ops/api.rules.yaml", findings)
	}
}

type memoryReader map[string]string

func (m memoryReader) ReadSource(file string) ([]byte, error) {
	return []byte(m[file]), nil
}

func TestRuleFileSeriesNames(t *testing.T) {
	files := memoryReader{
		"deploy/api.rules.yaml": "groups:\n  - rules:\n" +
			"      - record: job:http_errors:rate5m\n        expr: x\n" +
			"      - record: slo:api:error\n        labels: {slo: api}\n        expr: y\n" +
			"      - alert: APIErrors\n        expr: slo:api:error_rate > 1\n",
		"deploy/dashboards/api.json": `{"panels":[{"targets":[{"expr":"max_over_time(job:http_errors:rate5m[1h:5m])"}]}]}`,
	}
	tree := domain.Tree{Files: slices.Sorted(maps.Keys(files)), Reader: files}
	settings := newTestEngine().settings

	t.Run("a subquery step is not a series", func(t *testing.T) {
		if got := dashboardSeries(tree, domain.Graph{}, settings); len(got) != 0 {
			t.Fatalf("violations = %+v, want none", got)
		}
	})
	t.Run("an alert on a longer series name does not use the recorded series", func(t *testing.T) {
		if got := sloAlert(tree, domain.Graph{}, settings); len(got) != 1 {
			t.Fatalf("violations = %+v, want one", got)
		}
	})
}

func TestApplyHandlerNeedsExactName(t *testing.T) {
	const aggregate = "src/contexts/sales/domain/order.aggregate.ts"
	files := memoryReader{aggregate: "class Order {\n  place() { this.raise(new OrderPlaced(this.id)) }\n  reapplyOrderPlaced(e) {}\n}\n"}

	got := domain.ApplyHandler(domain.Tree{Files: []string{aggregate}, Reader: files}, domain.Graph{}, domain.Settings{})

	if want := []domain.Violation{{File: aggregate, Line: 2}}; !slices.Equal(got, want) {
		t.Fatalf("violations = %+v, want %+v", got, want)
	}
}

func ruleByID(pack domain.Pack, id string) domain.Rule {
	return pack.Rules[slices.IndexFunc(pack.Rules, func(rule domain.Rule) bool { return rule.ID == id })]
}
