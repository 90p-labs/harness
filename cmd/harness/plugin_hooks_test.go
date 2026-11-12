package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	hostcli "github.com/90p-labs/harness/internal/host/ui/cli"
)

const (
	claudeConfigDirEnv = "CLAUDE_CONFIG_DIR"
	userStopCommand    = "echo user-stop"
	// settingsWithHarnessHooks has two harness entries, one user hook and a key that is not a hook.
	settingsWithHarnessHooks = `{
  "permissions": {"allow": ["Bash(ls)"]},
  "hooks": {
    "Stop": [{"hooks": [
      {"type": "command", "command": "harness hook --host claude stop"},
      {"type": "command", "command": "` + userStopCommand + `"}
    ]}],
    "SubagentStop": [{"hooks": [{"type": "command", "command": "harness hook --host claude subagent-stop"}]}]
  }
}
`
)

// withClaudePlugin points Claude Code's config folder to one where the 90p plugin is installed and enabled.
func withClaudePlugin(t *testing.T) {
	t.Helper()
	configDir := t.TempDir()
	writeRepoFile(t, configDir, "settings.json", `{"enabledPlugins":{"90p@harness":true}}`)
	writeRepoFile(t, configDir, filepath.Join("plugins", "installed_plugins.json"),
		`{"version":2,"plugins":{"90p@harness":[{"scope":"user","installPath":"/plugins/90p"}]}}`)
	t.Setenv(claudeConfigDirEnv, configDir)
}

func TestInitWithThePluginSkipsTheHooks(t *testing.T) {
	t.Run("removes the harness entries and keeps the others", func(t *testing.T) {
		clearHarnessEnv(t)
		withClaudePlugin(t)
		dir := initRepo(t)
		writeRepoFile(t, dir, "go.mod", goModuleFile)
		writeRepoFile(t, dir, filepath.Join(".claude", "settings.json"), settingsWithHarnessHooks)

		code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)

		if code != exitOK || stderr != "" {
			t.Fatalf("code = %d, stderr = %q", code, stderr)
		}
		path := filepath.Join(dir, ".claude", "settings.json")
		if want := "host: skipped, the 90p plugin runs the hooks; removed 2 harness hooks from " + path + "\n"; !strings.Contains(stdout, want) {
			t.Fatalf("stdout = %q, want the line %q", stdout, want)
		}
		settings := readRepoFile(t, dir, filepath.Join(".claude", "settings.json"))
		if strings.Contains(settings, "harness hook") || !strings.Contains(settings, userStopCommand) ||
			!strings.Contains(settings, "Bash(ls)") || strings.Contains(settings, "SubagentStop") {
			t.Fatalf("settings = %s, want the user hook and the permissions only", settings)
		}
	})

	t.Run("writes no settings file", func(t *testing.T) {
		clearHarnessEnv(t)
		withClaudePlugin(t)
		dir := initRepo(t)
		writeRepoFile(t, dir, "go.mod", goModuleFile)

		code, stdout, stderr := execute(t, commandInit, initHostFlag, hostClaude)

		if code != exitOK || stderr != "" {
			t.Fatalf("code = %d, stderr = %q", code, stderr)
		}
		if !strings.Contains(stdout, "host: skipped, the 90p plugin runs the hooks\n") || strings.Contains(stdout, "host: wrote") {
			t.Fatalf("stdout = %q, want the skip line", stdout)
		}
		if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); !os.IsNotExist(err) {
			t.Fatalf(".claude/settings.json: %v, want no file", err)
		}
	})

	t.Run("still writes the codex hooks", func(t *testing.T) {
		clearHarnessEnv(t)
		withClaudePlugin(t)
		dir := initRepo(t)
		writeRepoFile(t, dir, "go.mod", goModuleFile)

		code, stdout, stderr := execute(t, commandInit, initHostFlag, "codex")

		if code != exitOK || stderr != "" {
			t.Fatalf("code = %d, stderr = %q", code, stderr)
		}
		if !strings.Contains(stdout, "host: wrote "+filepath.Join(dir, ".codex", "hooks.json")) {
			t.Fatalf("stdout = %q, want the codex hooks written", stdout)
		}
	})
}

func TestHostInstallWarnsWithThePlugin(t *testing.T) {
	clearHarnessEnv(t)
	clearHostEnv(t)
	withClaudePlugin(t)
	dir := initRepo(t)
	path := filepath.Join(dir, ".claude", "settings.json")

	code, stdout, stderr := execute(t, "host", "install", initHostFlag, hostClaude)
	if code != exitOK || strings.TrimSpace(stdout) != path || stderr != hostcli.PluginWarning+"\n" {
		t.Fatalf("install: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}

	code, stdout, stderr = execute(t, "host", "uninstall", initHostFlag, hostClaude)
	if code != exitOK || !strings.HasPrefix(stdout, "removed ") || !strings.HasSuffix(stdout, " harness hooks from "+path+"\n") {
		t.Fatalf("uninstall: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
	if settings := readRepoFile(t, dir, filepath.Join(".claude", "settings.json")); strings.Contains(settings, "harness hook") {
		t.Fatalf("settings after uninstall = %s", settings)
	}
}

func TestHostInstallWithoutThePluginPrintsNoWarning(t *testing.T) {
	clearHarnessEnv(t)
	clearHostEnv(t)
	initRepo(t)
	if code, _, stderr := execute(t, "host", "install", initHostFlag, hostClaude); code != exitOK || stderr != "" {
		t.Fatalf("code = %d, stderr = %q", code, stderr)
	}
}
