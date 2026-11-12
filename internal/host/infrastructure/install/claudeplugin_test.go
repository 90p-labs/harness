package install_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/90p-labs/harness/internal/host/infrastructure/install"
)

const (
	pluginKey         = pluginName + "@" + marketplaceName
	installedRegistry = `{"version":2,"plugins":{"` + pluginKey + `":[{"scope":"user","installPath":"/x"}]}}`
	enabledPlugin     = `{"enabledPlugins":{"` + pluginKey + `":true}}`
	disabledPlugin    = `{"enabledPlugins":{"` + pluginKey + `":false}}`
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestClaudePlugin(t *testing.T) {
	type files map[string]string
	cases := []struct {
		name     string
		config   files
		worktree files
		want     bool
	}{
		{name: "nothing installed", want: false},
		{
			name:   "installed and enabled for the user",
			config: files{"plugins/installed_plugins.json": installedRegistry, "settings.json": enabledPlugin},
			want:   true,
		},
		{
			name:   "installed but not enabled",
			config: files{"plugins/installed_plugins.json": installedRegistry, "settings.json": `{}`},
			want:   false,
		},
		{
			name:   "enabled but not installed",
			config: files{"settings.json": enabledPlugin},
			want:   false,
		},
		{
			name:     "enabled in the project settings",
			config:   files{"plugins/installed_plugins.json": installedRegistry},
			worktree: files{".claude/settings.json": enabledPlugin},
			want:     true,
		},
		{
			name:     "disabled in the local settings",
			config:   files{"plugins/installed_plugins.json": installedRegistry, "settings.json": enabledPlugin},
			worktree: files{".claude/settings.local.json": disabledPlugin},
			want:     false,
		},
		{
			name:   "another marketplace with the same plugin name",
			config: files{"plugins/installed_plugins.json": `{"plugins":{"90p@fork":[{"scope":"user"}]}}`, "settings.json": `{"enabledPlugins":{"90p@fork":true}}`},
			want:   true,
		},
		{
			name:   "another plugin",
			config: files{"plugins/installed_plugins.json": `{"plugins":{"other@harness":[{"scope":"user"}]}}`, "settings.json": `{"enabledPlugins":{"other@harness":true}}`},
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configDir, worktree := t.TempDir(), t.TempDir()
			for rel, content := range tc.config {
				writeFile(t, filepath.Join(configDir, rel), content)
			}
			for rel, content := range tc.worktree {
				writeFile(t, filepath.Join(worktree, rel), content)
			}
			getenv := func(key string) string {
				if key == "CLAUDE_CONFIG_DIR" {
					return configDir
				}
				return ""
			}
			got, err := install.ClaudePlugin(worktree, getenv)
			if err != nil || got != tc.want {
				t.Fatalf("ClaudePlugin = %v, %v, want %v", got, err, tc.want)
			}
		})
	}

	t.Run("home without a config dir", func(t *testing.T) {
		home := t.TempDir()
		writeFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), installedRegistry)
		writeFile(t, filepath.Join(home, ".claude", "settings.json"), enabledPlugin)
		getenv := func(key string) string {
			if key == "HOME" {
				return home
			}
			return ""
		}
		if got, err := install.ClaudePlugin(t.TempDir(), getenv); err != nil || !got {
			t.Fatalf("ClaudePlugin = %v, %v, want true", got, err)
		}
	})

	t.Run("no home and no config dir", func(t *testing.T) {
		if got, err := install.ClaudePlugin(t.TempDir(), func(string) string { return "" }); err != nil || got {
			t.Fatalf("ClaudePlugin = %v, %v, want false", got, err)
		}
	})

	t.Run("malformed settings", func(t *testing.T) {
		configDir := t.TempDir()
		writeFile(t, filepath.Join(configDir, "settings.json"), `{`)
		getenv := func(string) string { return configDir }
		if _, err := install.ClaudePlugin(t.TempDir(), getenv); err == nil {
			t.Fatal("want an error for malformed settings")
		}
	})

	t.Run("malformed registry", func(t *testing.T) {
		configDir := t.TempDir()
		writeFile(t, filepath.Join(configDir, "settings.json"), enabledPlugin)
		writeFile(t, filepath.Join(configDir, "plugins", "installed_plugins.json"), `[`)
		getenv := func(string) string { return configDir }
		if _, err := install.ClaudePlugin(t.TempDir(), getenv); err == nil {
			t.Fatal("want an error for a malformed registry")
		}
	})
}

func TestUninstall(t *testing.T) {
	t.Run("removes the harness entries and keeps the others", func(t *testing.T) {
		worktree := t.TempDir()
		path := filepath.Join(worktree, ".claude", "settings.json")
		writeFile(t, path, `{
  "permissions": {"allow": ["Bash(ls)"]},
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "harness hook --host claude stop"}, {"type": "command", "command": "`+userCommand+`"}]}
    ],
    "SubagentStop": [
      {"hooks": [{"type": "command", "command": "harness hook --host claude subagent-stop"}]}
    ],
    "Notification": [
      {"hooks": [{"type": "command", "command": "`+notifyCommand+`"}]}
    ]
  }
}
`)
		got, removed, err := install.Uninstall(worktree, "claude")
		if err != nil || got != path || removed != 2 {
			t.Fatalf("Uninstall = %q, %d, %v, want %q, 2", got, removed, err, path)
		}
		s, _ := readSettings(t, path)
		if c := commandsOf(s, "Stop"); len(c) != 1 || c[0] != userCommand {
			t.Errorf("Stop = %v, want only the user hook", c)
		}
		if _, ok := s.Hooks["SubagentStop"]; ok {
			t.Errorf("SubagentStop kept with no hooks: %v", s.Hooks["SubagentStop"])
		}
		if c := commandsOf(s, "Notification"); len(c) != 1 || c[0] != notifyCommand {
			t.Errorf("Notification = %v, want the user hook", c)
		}
		if string(s.Permissions) == "" {
			t.Error("permissions dropped")
		}
	})

	t.Run("drops the hooks key left empty", func(t *testing.T) {
		worktree := t.TempDir()
		path := filepath.Join(worktree, ".claude", "settings.json")
		writeFile(t, path, `{"model":"opus","hooks":{"Stop":[{"hooks":[{"type":"command","command":"harness hook --host claude stop"}]}]}}`)
		if _, removed, err := install.Uninstall(worktree, "claude"); err != nil || removed != 1 {
			t.Fatalf("Uninstall = %d, %v, want 1", removed, err)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != "{\n  \"model\": \"opus\"\n}\n" {
			t.Fatalf("settings = %s", raw)
		}
	})

	t.Run("leaves a file without harness hooks as it is", func(t *testing.T) {
		worktree := t.TempDir()
		path := filepath.Join(worktree, ".claude", "settings.json")
		content := `{"hooks":{"Stop":[{"hooks":[{"command":"` + userCommand + `"}]}]}}`
		writeFile(t, path, content)
		if _, removed, err := install.Uninstall(worktree, "claude"); err != nil || removed != 0 {
			t.Fatalf("Uninstall = %d, %v, want 0", removed, err)
		}
		if raw, _ := os.ReadFile(path); string(raw) != content {
			t.Fatalf("settings rewritten: %s", raw)
		}
	})

	t.Run("a missing file", func(t *testing.T) {
		worktree := t.TempDir()
		if _, removed, err := install.Uninstall(worktree, "claude"); err != nil || removed != 0 {
			t.Fatalf("Uninstall = %d, %v, want 0", removed, err)
		}
		if _, err := os.Stat(filepath.Join(worktree, ".claude")); !os.IsNotExist(err) {
			t.Fatalf("created .claude: %v", err)
		}
	})

	t.Run("an unknown host", func(t *testing.T) {
		if _, _, err := install.Uninstall(t.TempDir(), "vim"); err == nil {
			t.Fatal("want an error")
		}
	})

	t.Run("malformed hooks", func(t *testing.T) {
		worktree := t.TempDir()
		writeFile(t, filepath.Join(worktree, ".claude", "settings.json"), `{"hooks":[]}`)
		if _, _, err := install.Uninstall(worktree, "claude"); err == nil {
			t.Fatal("want an error")
		}
	})
}
