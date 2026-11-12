package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	claudePluginName    = "90p"
	claudeConfigDirEnv  = "CLAUDE_CONFIG_DIR"
	homeEnv             = "HOME"
	claudeHomeDir       = ".claude"
	claudeLocalFile     = "settings.local.json"
	claudePluginsDir    = "plugins"
	claudeInstalledFile = "installed_plugins.json"
	pluginKeySeparator  = "@"
)

// ClaudePlugin reports whether Claude Code runs the hooks of the 90p plugin in the worktree. It reads what Claude Code
// itself reads: the plugin must be enabled in the merged enabledPlugins of the user, project and local settings, and
// listed in plugins/installed_plugins.json of the Claude config folder (CLAUDE_CONFIG_DIR, else ~/.claude).
func ClaudePlugin(worktree string, getenv func(string) string) (bool, error) {
	configDir := getenv(claudeConfigDirEnv)
	if configDir == "" {
		home := getenv(homeEnv)
		if home == "" {
			return false, nil
		}
		configDir = filepath.Join(home, claudeHomeDir)
	}
	enabled := map[string]bool{}
	for _, path := range []string{
		filepath.Join(configDir, claudeSettingsFile),
		filepath.Join(worktree, claudeSettingsDir, claudeSettingsFile),
		filepath.Join(worktree, claudeSettingsDir, claudeLocalFile),
	} {
		var settings struct {
			EnabledPlugins map[string]bool `json:"enabledPlugins"`
		}
		found, err := readJSON(path, &settings)
		if err != nil {
			return false, err
		}
		if !found {
			continue
		}
		for key, on := range settings.EnabledPlugins {
			enabled[key] = on
		}
	}
	var keys []string
	for key, on := range enabled {
		if on && isHarnessPlugin(key) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return false, nil
	}
	var registry struct {
		Plugins map[string][]json.RawMessage `json:"plugins"`
	}
	if _, err := readJSON(filepath.Join(configDir, claudePluginsDir, claudeInstalledFile), &registry); err != nil {
		return false, err
	}
	for _, key := range keys {
		if len(registry.Plugins[key]) > 0 {
			return true, nil
		}
	}
	return false, nil
}

func isHarnessPlugin(key string) bool {
	name, _, _ := strings.Cut(key, pluginKeySeparator)
	return name == claudePluginName
}

// readJSON decodes the file into v. A missing file is not an error and reports false.
func readJSON(path string, v any) (bool, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	return true, nil
}
