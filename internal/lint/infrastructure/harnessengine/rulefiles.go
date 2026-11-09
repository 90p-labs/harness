package harnessengine

import (
	"encoding/json"
	"path"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/90p-labs/harness/internal/lint/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/git"
)

const (
	ruleTestSuffix = ".rules.test.yaml"
	sloLabel       = "slo"
	runbookKey     = "runbook_url"
	exprKey        = "expr"
)

var (
	recordingRuleName = regexp.MustCompile(`\b[a-zA-Z_]\w*(?::\w+)+`)
	seriesToken       = regexp.MustCompile(`[a-zA-Z_:][\w:]*`)
)

func usesSeries(name string) func(expr string) bool {
	return func(expr string) bool { return slices.Contains(seriesToken.FindAllString(expr, -1), name) }
}

type promRule struct {
	Alert       string            `yaml:"alert"`
	Record      string            `yaml:"record"`
	Expr        string            `yaml:"expr"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

type promRuleFile struct {
	Groups []struct {
		Rules []promRule `yaml:"rules"`
	} `yaml:"groups"`
}

type promTestFile struct {
	RuleFiles []string `yaml:"rule_files"`
	Tests     []struct {
		AlertRuleTest []struct {
			Alertname string `yaml:"alertname"`
		} `yaml:"alert_rule_test"`
		PromQLExprTest []struct {
			Expr string `yaml:"expr"`
		} `yaml:"promql_expr_test"`
	} `yaml:"tests"`
}

type ruleSource struct {
	file  string
	rules []promRule
}

type testCoverage struct {
	alerts []string
	exprs  []string
}

func ruleTests(tree domain.Tree, _ domain.Graph, settings domain.Settings) []domain.Violation {
	coverage := map[string]*testCoverage{}
	for _, file := range tree.Files {
		var parsed promTestFile
		if !strings.HasSuffix(file, ruleTestSuffix) || !readYAML(tree, file, &parsed) {
			continue
		}
		for _, ruleFile := range parsed.RuleFiles {
			target := path.Join(path.Dir(file), ruleFile)
			if coverage[target] == nil {
				coverage[target] = &testCoverage{}
			}
			for _, test := range parsed.Tests {
				for _, alert := range test.AlertRuleTest {
					coverage[target].alerts = append(coverage[target].alerts, alert.Alertname)
				}
				for _, expr := range test.PromQLExprTest {
					coverage[target].exprs = append(coverage[target].exprs, expr.Expr)
				}
			}
		}
	}
	var found []domain.Violation
	for _, source := range ruleSources(tree, settings) {
		covered := coverage[source.file]
		if covered == nil || slices.ContainsFunc(source.rules, func(rule promRule) bool { return !covered.names(rule) }) {
			found = append(found, domain.Violation{File: source.file})
		}
	}
	return found
}

func (c *testCoverage) names(rule promRule) bool {
	if rule.Alert != "" {
		return slices.Contains(c.alerts, rule.Alert)
	}
	return slices.ContainsFunc(c.exprs, usesSeries(rule.Record))
}

func sloAlert(tree domain.Tree, _ domain.Graph, settings domain.Settings) []domain.Violation {
	sources := ruleSources(tree, settings)
	var alertExprs []string
	for _, source := range sources {
		for _, rule := range source.rules {
			if rule.Alert != "" {
				alertExprs = append(alertExprs, rule.Expr)
			}
		}
	}
	var found []domain.Violation
	for _, source := range sources {
		for _, rule := range source.rules {
			_, slo := rule.Labels[sloLabel]
			used := slices.ContainsFunc(alertExprs, usesSeries(rule.Record))
			if rule.Record != "" && slo && !used {
				found = append(found, domain.Violation{File: source.file})
			}
		}
	}
	return found
}

func runbook(tree domain.Tree, _ domain.Graph, settings domain.Settings) []domain.Violation {
	var found []domain.Violation
	for _, source := range ruleSources(tree, settings) {
		for _, rule := range source.rules {
			if rule.Alert != "" && rule.Annotations[runbookKey] == "" {
				found = append(found, domain.Violation{File: source.file})
			}
		}
	}
	return found
}

func dashboardSeries(tree domain.Tree, _ domain.Graph, settings domain.Settings) []domain.Violation {
	var recorded []string
	for _, source := range ruleSources(tree, settings) {
		for _, rule := range source.rules {
			recorded = append(recorded, rule.Record)
		}
	}
	var found []domain.Violation
	for _, file := range tree.Files {
		if !git.MatchesAny([]string{settings.Observability.Dashboards}, file) {
			continue
		}
		source, err := tree.Source(file)
		var dashboard any
		if err != nil || json.Unmarshal(source, &dashboard) != nil {
			continue
		}
		var missing []string
		for _, expr := range dashboardExpressions(dashboard) {
			for _, series := range recordingRuleName.FindAllString(expr, -1) {
				if !slices.Contains(recorded, series) && !slices.Contains(missing, series) {
					missing = append(missing, series)
					found = append(found, domain.Violation{File: file})
				}
			}
		}
	}
	return found
}

func dashboardExpressions(node any) []string {
	var exprs []string
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if text, ok := child.(string); ok && key == exprKey {
				exprs = append(exprs, text)
				continue
			}
			exprs = append(exprs, dashboardExpressions(child)...)
		}
	case []any:
		for _, child := range value {
			exprs = append(exprs, dashboardExpressions(child)...)
		}
	}
	return exprs
}

func ruleSources(tree domain.Tree, settings domain.Settings) []ruleSource {
	var sources []ruleSource
	for _, file := range tree.Files {
		var parsed promRuleFile
		if !git.MatchesAny([]string{settings.Observability.Rules}, file) || !readYAML(tree, file, &parsed) {
			continue
		}
		source := ruleSource{file: file}
		for _, group := range parsed.Groups {
			source.rules = append(source.rules, group.Rules...)
		}
		sources = append(sources, source)
	}
	return sources
}

func readYAML(tree domain.Tree, file string, into any) bool {
	source, err := tree.Source(file)
	return err == nil && yaml.Unmarshal(source, into) == nil
}
