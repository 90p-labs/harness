package biome

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/jorge07/harness/internal/lint/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	pluginCategory        = "plugin"
	internalErrorCategory = "internalError"
)

var messagePrefix = regexp.MustCompile(`^\[([^\]]+)\]\s*`)

type report struct {
	Diagnostics []diagnostic `json:"diagnostics"`
}

type diagnostic struct {
	Message  string `json:"message"`
	Category string `json:"category"`
	Location struct {
		Path  string `json:"path"`
		Start struct {
			Line int `json:"line"`
		} `json:"start"`
	} `json:"location"`
}

// MapReport keeps the diagnostics of the given rules and fails on a Biome internal error. It ignores any other diagnostic, such as a parse error.
func MapReport(raw []byte, rules []domain.Rule) ([]shareddomain.Finding, error) {
	var parsed report
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("decode biome report: %w", err)
	}
	byID := map[string]domain.Rule{}
	byBiome := map[string]domain.Rule{}
	for _, rule := range rules {
		byID[rule.ID] = rule
		if rule.BiomeRule != "" {
			byBiome[rule.BiomeRule] = rule
		}
	}
	var findings []shareddomain.Finding
	for _, d := range parsed.Diagnostics {
		if strings.HasPrefix(d.Category, internalErrorCategory) {
			return nil, fmt.Errorf("biome %s on %s: %s", d.Category, d.Location.Path, d.Message)
		}
		rule, message, ok := match(d, byID, byBiome)
		if !ok {
			continue
		}
		findings = append(findings, shareddomain.Finding{
			Rule:     rule.ID,
			Severity: rule.Severity,
			File:     d.Location.Path,
			Line:     d.Location.Start.Line,
			Message:  message,
		})
	}
	return findings, nil
}

func match(d diagnostic, byID, byBiome map[string]domain.Rule) (domain.Rule, string, bool) {
	if d.Category != pluginCategory {
		rule, ok := byBiome[d.Category]
		return rule, d.Message, ok
	}
	groups := messagePrefix.FindStringSubmatch(d.Message)
	if groups == nil {
		return domain.Rule{}, "", false
	}
	rule, ok := byID[groups[1]]
	return rule, d.Message[len(groups[0]):], ok
}
