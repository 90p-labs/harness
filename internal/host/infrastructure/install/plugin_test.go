package install_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/90p-labs/harness/internal/host/domain"
	"github.com/90p-labs/harness/internal/host/infrastructure/install"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"

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
	pluginName        = "90p"
	marketplaceName   = "harness"
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

	if claude.Name != pluginName {
		t.Errorf("plugin name = %q, want %q", claude.Name, pluginName)
	}

	var claudeMarket struct {
		Name    string `json:"name"`
		Plugins []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"plugins"`
	}
	readJSON(t, claudeMarketplace, &claudeMarket)
	if len(claudeMarket.Plugins) != 1 || claudeMarket.Plugins[0].Name != claude.Name || claudeMarket.Plugins[0].Source != repositoryRoot {
		t.Errorf("%s plugins = %+v, want %s at %s", claudeMarketplace, claudeMarket.Plugins, claude.Name, repositoryRoot)
	}

	if claudeMarket.Name != marketplaceName {
		t.Errorf("%s name = %q, want %q", claudeMarketplace, claudeMarket.Name, marketplaceName)
	}

	var codexMarket struct {
		Name    string `json:"name"`
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

	if codexMarket.Name != marketplaceName {
		t.Errorf("%s name = %q, want %q", codexMarketplace, codexMarket.Name, marketplaceName)
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

type matcherFile struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
	} `json:"hooks"`
}

func readMatchers(t *testing.T, path string) matcherFile {
	t.Helper()
	var file matcherFile
	readJSON(t, path, &file)
	return file
}

func TestClaudeHooksMatchRuleTools(t *testing.T) {
	want := domain.ClaudeMatcher()
	for _, path := range []string{claudeHooksFile, "internal/host/infrastructure/install/claude.json"} {
		file := readMatchers(t, path)
		for event, groups := range file.Hooks {
			wantMatcher := ""
			if event == "PreToolUse" || event == "PostToolUse" {
				wantMatcher = want
			}
			for _, group := range groups {
				if group.Matcher != wantMatcher {
					t.Errorf("%s %s matcher = %q, want %q", path, event, group.Matcher, wantMatcher)
				}
			}
		}
	}

	unmatched := readMatchers(t, "internal/host/infrastructure/install/claude_all.json")
	for event, groups := range unmatched.Hooks {
		for _, group := range groups {
			if group.Matcher != "" {
				t.Errorf("claude_all.json %s matcher = %q, want none", event, group.Matcher)
			}
		}
	}

	for _, matched := range []bool{true, false} {
		worktree := t.TempDir()
		path, err := install.Install(worktree, shareddomain.HostClaude, matched)
		if err != nil {
			t.Fatal(err)
		}
		s, _ := readSettings(t, path)
		for _, event := range []string{"PreToolUse", "PostToolUse"} {
			wantMatcher := ""
			if matched {
				wantMatcher = want
			}
			if got := s.Hooks[event][0].Matcher; got != wantMatcher {
				t.Errorf("matched=%v %s matcher = %q, want %q", matched, event, got, wantMatcher)
			}
		}
		if got := s.Hooks["Stop"][0].Matcher; got != "" {
			t.Errorf("matched=%v Stop matcher = %q, want none", matched, got)
		}
	}

	codex := t.TempDir()
	path, err := install.Install(codex, shareddomain.HostCodex, true)
	if err != nil {
		t.Fatal(err)
	}
	s, _ := readSettings(t, path)
	for event, groups := range s.Hooks {
		for _, group := range groups {
			if group.Matcher != "" {
				t.Errorf("codex %s matcher = %q, want none", event, group.Matcher)
			}
		}
	}
}
