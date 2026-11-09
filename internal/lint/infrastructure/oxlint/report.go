package oxlint

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/90p-labs/harness/internal/lint/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

// oxlint prints the plugin of a rule under another name than the config uses.
var codeScopes = map[string]string{"nextjs": "next"}

type report struct {
	Diagnostics []diagnostic `json:"diagnostics"`
}

type diagnostic struct {
	Code     string  `json:"code"`
	Filename string  `json:"filename"`
	Labels   []label `json:"labels"`
}

type label struct {
	Span struct {
		Line int `json:"line"`
	} `json:"span"`
}

func parseReport(raw []byte) (report, error) {
	var parsed report
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return report{}, fmt.Errorf("decode oxlint report: %w", err)
	}
	return parsed, nil
}

func codeOf(oxlintRule string) string {
	plugin, name, _ := strings.Cut(oxlintRule, ruleSeparator)
	if scope, ok := codeScopes[plugin]; ok {
		plugin = scope
	}
	return plugin + "(" + name + ")"
}

func (r report) findings(dir string, rules []domain.Rule) []shareddomain.Finding {
	byCode := make(map[string]domain.Rule, len(rules))
	for _, rule := range rules {
		byCode[codeOf(rule.OxlintRule)] = rule
	}
	var findings []shareddomain.Finding
	for _, item := range r.Diagnostics {
		rule, ok := byCode[item.Code]
		if !ok {
			continue
		}
		finding := shareddomain.Finding{
			Rule:     rule.ID,
			Severity: rule.Severity,
			File:     relativeTo(dir, item.Filename),
			Message:  rule.Message,
		}
		if len(item.Labels) > 0 {
			finding.Line = item.Labels[0].Span.Line
		}
		findings = append(findings, finding)
	}
	return findings
}

func relativeTo(dir, name string) string {
	if !filepath.IsAbs(name) {
		return filepath.ToSlash(name)
	}
	rel, err := filepath.Rel(dir, name)
	if err != nil {
		return filepath.ToSlash(name)
	}
	return filepath.ToSlash(rel)
}
