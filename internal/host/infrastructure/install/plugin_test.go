package install_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	claudeManifest    = ".claude-plugin/plugin.json"
	claudeMarketplace = ".claude-plugin/marketplace.json"
	codexManifest     = ".codex-plugin/plugin.json"
	codexMarketplace  = ".agents/plugins/marketplace.json"
	claudeHooksFile   = "hooks/hooks.json"
	codexHooksFile    = "hooks/codex.json"
	repositoryRoot    = "./"
)

type manifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Skills  string `json:"skills"`
	Hooks   string `json:"hooks"`
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	raw, err := os.ReadFile(repository.Path(t, path))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func TestPluginManifests(t *testing.T) {
	var claude, codex manifest
	readJSON(t, claudeManifest, &claude)
	readJSON(t, codexManifest, &codex)
	if claude.Version == "" || claude.Version != codex.Version || claude.Name != codex.Name {
		t.Errorf("claude = %s %s, codex = %s %s, want one name and one version", claude.Name, claude.Version, codex.Name, codex.Version)
	}

	var claudeMarket struct {
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	readJSON(t, claudeMarketplace, &claudeMarket)
	if len(claudeMarket.Plugins) != 1 || claudeMarket.Plugins[0].Name != claude.Name || claudeMarket.Plugins[0].Source != repositoryRoot {
		t.Errorf("%s plugins = %+v, want %s at %s", claudeMarketplace, claudeMarket.Plugins, claude.Name, repositoryRoot)
	}

	var codexMarket struct {
		Plugins []struct {
			Name   string `json:"name"`
			Source struct {
				Path string `json:"path"`
			} `json:"source"`
		} `json:"plugins"`
	}
	readJSON(t, codexMarketplace, &codexMarket)
	if len(codexMarket.Plugins) != 1 || codexMarket.Plugins[0].Name != codex.Name || codexMarket.Plugins[0].Source.Path != repositoryRoot {
		t.Errorf("%s plugins = %+v, want %s at %s", codexMarketplace, codexMarket.Plugins, codex.Name, repositoryRoot)
	}

	if filepath.Clean(codex.Hooks) != codexHooksFile {
		t.Errorf("%s hooks = %q, want %s", codexManifest, codex.Hooks, codexHooksFile)
	}
	for _, path := range []string{codex.Skills, codex.Hooks, claudeHooksFile} {
		if path == "" {
			t.Errorf("%s names an empty path", codexManifest)
			continue
		}
		if _, err := os.Stat(repository.Path(t, path)); err != nil {
			t.Errorf("manifest path %q: %v", path, err)
		}
	}
}
