package biome

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/90p-labs/harness/internal/lint/domain"
)

const (
	pluginsDir      = "plugins"
	configsDir      = "biome"
	configFile      = "biome.json"
	scopePrefix     = "s"
	scopeHexLength  = 8
	builtinPrefix   = "lint/"
	builtinSeverity = "error"
	presetNone      = "none"
	dirMode         = 0o755
	fileMode        = 0o644
)

type toggle struct {
	Enabled bool `json:"enabled"`
}

type linterConfig struct {
	Enabled bool           `json:"enabled"`
	Rules   map[string]any `json:"rules"`
}

type scopeConfig struct {
	Root      bool         `json:"root"`
	Formatter toggle       `json:"formatter"`
	Assist    toggle       `json:"assist"`
	Linter    linterConfig `json:"linter"`
	Plugins   []string     `json:"plugins,omitempty"`
}

func scopeID(glob string) string {
	sum := sha256.Sum256([]byte(glob))
	return scopePrefix + hex.EncodeToString(sum[:])[:scopeHexLength]
}

func scopeDir(glob string) string {
	return path.Join(domain.LintDir, configsDir, scopeID(glob))
}

func groupByScope(rules []domain.Rule) map[string][]domain.Rule {
	scopes := map[string][]domain.Rule{}
	for _, rule := range rules {
		scopes[rule.Scope] = append(scopes[rule.Scope], rule)
	}
	return scopes
}

func buildConfig(rules []domain.Rule) scopeConfig {
	linterRules := map[string]any{"preset": presetNone}
	config := scopeConfig{Linter: linterConfig{Enabled: true, Rules: linterRules}, Root: true}
	for _, rule := range rules {
		if rule.Plugin != "" {
			config.Plugins = append(config.Plugins, pluginRelativePath(rule.Plugin))
		}
		if rule.BiomeRule != "" {
			group, name, _ := strings.Cut(strings.TrimPrefix(rule.BiomeRule, builtinPrefix), "/")
			groupRules, _ := linterRules[group].(map[string]string)
			if groupRules == nil {
				groupRules = map[string]string{}
				linterRules[group] = groupRules
			}
			groupRules[name] = builtinSeverity
		}
	}
	return config
}

func pluginFile(embedded string) string {
	pack, _, _ := strings.Cut(embedded, "/")
	return path.Join(pluginsDir, pack, path.Base(embedded))
}

func pluginTarget(embedded string) string {
	return path.Join(domain.LintDir, pluginFile(embedded))
}

func pluginRelativePath(embedded string) string {
	return path.Join("..", "..", pluginFile(embedded))
}

func writePlugin(plugins fs.FS, dir, embedded string) error {
	source, err := fs.ReadFile(plugins, embedded)
	if err != nil {
		return fmt.Errorf("read plugin %s: %w", embedded, err)
	}
	return writeFile(filepath.Join(dir, filepath.FromSlash(pluginTarget(embedded))), source)
}

func writeConfig(dir, glob string, config scopeConfig) error {
	raw, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, filepath.FromSlash(scopeDir(glob)), configFile), raw)
}

func writeFile(target string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), dirMode); err != nil {
		return err
	}
	return os.WriteFile(target, content, fileMode)
}
