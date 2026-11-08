package oxlint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jorge07/harness/internal/lint/domain"
)

const (
	configFile     = "oxlintrc.json"
	ruleSeparator  = "/"
	levelError     = "error"
	levelOff       = "off"
	categoryName   = "correctness"
	dirMode        = 0o755
	fileMode       = 0o644
	configIndent   = "  "
	configNewline  = "\n"
	noConfigPrefix = ""
	// In 1.92.0 an override glob in .harness-lint/ does not match src/** without this prefix.
	anyDepth = "**/"
)

type config struct {
	Categories map[string]string `json:"categories"`
	Overrides  []override        `json:"overrides"`
}

type override struct {
	Files   []string          `json:"files"`
	Plugins []string          `json:"plugins"`
	Rules   map[string]string `json:"rules"`
}

func configPath(dir string) string {
	return filepath.Join(dir, domain.LintDir, configFile)
}

func buildConfig(rules []domain.Rule) config {
	byScope := map[string]*override{}
	var scopes []string
	for _, rule := range rules {
		entry, ok := byScope[rule.Scope]
		if !ok {
			entry = &override{Files: []string{anyDepth + rule.Scope}, Plugins: []string{}, Rules: map[string]string{}}
			byScope[rule.Scope] = entry
			scopes = append(scopes, rule.Scope)
		}
		plugin, _, _ := strings.Cut(rule.OxlintRule, ruleSeparator)
		if !slices.Contains(entry.Plugins, plugin) {
			entry.Plugins = append(entry.Plugins, plugin)
		}
		entry.Rules[rule.OxlintRule] = levelError
	}
	slices.Sort(scopes)
	cfg := config{Categories: map[string]string{categoryName: levelOff}, Overrides: []override{}}
	for _, scope := range scopes {
		cfg.Overrides = append(cfg.Overrides, *byScope[scope])
	}
	return cfg
}

func writeConfig(dir string, rules []domain.Rule) error {
	raw, err := json.MarshalIndent(buildConfig(rules), noConfigPrefix, configIndent)
	if err != nil {
		return fmt.Errorf("encode oxlint config: %w", err)
	}
	path := configPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, append(raw, configNewline...), fileMode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}
