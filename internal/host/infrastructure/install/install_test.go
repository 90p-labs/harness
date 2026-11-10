package install_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/host/infrastructure/install"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	userCommand   = "echo user-hook"
	notifyCommand = "echo notify && echo done"
	auditCommand  = "harness hooks-audit"
	harnessPrefix = "harness hook "
	userFileMode  = 0o600
)

var hookEvents = map[shareddomain.Host]int{
	shareddomain.HostClaude: 9,
	shareddomain.HostCodex:  7,
}

type settings struct {
	Hooks map[string][]struct {
		Matcher string `json:"matcher"`
		Hooks   []struct {
			Command string `json:"command"`
		} `json:"hooks"`
	} `json:"hooks"`
	Permissions json.RawMessage `json:"permissions"`
}

func readSettings(t *testing.T, path string) (settings, []byte) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var s settings
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	return s, raw
}

func commandsOf(s settings, event string) []string {
	var out []string
	for _, group := range s.Hooks[event] {
		for _, h := range group.Hooks {
			out = append(out, h.Command)
		}
	}
	return out
}

func countPrefixed(commands []string) int {
	n := 0
	for _, c := range commands {
		if strings.HasPrefix(c, harnessPrefix) {
			n++
		}
	}
	return n
}

func checkInstall(t *testing.T, host shareddomain.Host, relPath string) {
	t.Helper()

	t.Run("creates the file in a worktree without it", func(t *testing.T) {
		worktree := t.TempDir()
		path, err := install.Install(worktree, host)
		if err != nil {
			t.Fatal(err)
		}
		if path != filepath.Join(worktree, relPath) {
			t.Fatalf("path = %q, want %q", path, filepath.Join(worktree, relPath))
		}
		s, raw := readSettings(t, path)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Fatalf("file mode = %v, want 0644", info.Mode().Perm())
		}
		if len(s.Hooks) != hookEvents[host] {
			t.Fatalf("events = %d, want %d", len(s.Hooks), hookEvents[host])
		}
		if !bytes.HasSuffix(raw, []byte("\n")) || !bytes.Contains(raw, []byte("\n  \"hooks\"")) {
			t.Fatalf("file is not indented by 2 with a trailing newline:\n%s", raw)
		}
		wantHost := "--host " + string(host) + " "
		if !strings.Contains(string(raw), wantHost) {
			t.Fatalf("commands miss %q:\n%s", wantHost, raw)
		}
	})

	t.Run("merges with user hooks and is idempotent", func(t *testing.T) {
		worktree := t.TempDir()
		path := filepath.Join(worktree, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		existing := `{
  "permissions": {"allow": ["Bash(ls)"]},
  "hooks": {
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [
        {"type": "command", "command": "echo user-hook"},
        {"type": "command", "command": "harness hooks-audit"},
        {"type": "command", "command": "harness hook --host old pre-tool-use"}
      ]},
      {"hooks": [{"type": "command", "command": "harness hook"}]}
    ],
    "Notification": [{"hooks": [{"type": "command", "command": "echo notify && echo done"}]}, {"matcher": "idle"}]
  }
}`
		if err := os.WriteFile(path, []byte(existing), userFileMode); err != nil {
			t.Fatal(err)
		}

		if _, err := install.Install(worktree, host); err != nil {
			t.Fatal(err)
		}
		s, first := readSettings(t, path)

		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != userFileMode {
			t.Fatalf("file mode = %v, want the user mode %v kept", info.Mode().Perm(), os.FileMode(userFileMode))
		}
		pre := commandsOf(s, "PreToolUse")
		if countPrefixed(pre) != 1 {
			t.Fatalf("PreToolUse harness hooks = %v, want exactly one", pre)
		}
		if len(pre) != 3 || pre[0] != userCommand || pre[1] != auditCommand {
			t.Fatalf("PreToolUse commands = %v, want the user hooks kept first", pre)
		}
		if s.Hooks["PreToolUse"][0].Matcher != "Bash" {
			t.Fatalf("user matcher lost: %+v", s.Hooks["PreToolUse"])
		}
		if len(s.Hooks["PreToolUse"]) != 2 {
			t.Fatalf("PreToolUse groups = %d, want 2 (empty group dropped)", len(s.Hooks["PreToolUse"]))
		}
		if got := commandsOf(s, "Notification"); len(got) != 1 || !bytes.Contains(first, []byte(notifyCommand)) {
			t.Fatalf("Notification = %v, want %q untouched in the file:\n%s", got, notifyCommand, first)
		}
		var raw struct {
			Hooks map[string][]map[string]json.RawMessage `json:"hooks"`
		}
		if err := json.Unmarshal(first, &raw); err != nil {
			t.Fatal(err)
		}
		if groups := raw.Hooks["Notification"]; len(groups) != 2 || len(groups[1]) != 1 {
			t.Fatalf("Notification groups = %v, want the matcher-only group unchanged", groups)
		}
		if !strings.Contains(string(s.Permissions), "Bash(ls)") {
			t.Fatalf("permissions lost: %s", s.Permissions)
		}
		if len(s.Hooks) != hookEvents[host]+1 {
			t.Fatalf("events = %d, want %d", len(s.Hooks), hookEvents[host]+1)
		}

		if _, err := install.Install(worktree, host); err != nil {
			t.Fatal(err)
		}
		_, second := readSettings(t, path)
		if !bytes.Equal(first, second) {
			t.Fatalf("second run changed the file:\n%s\n---\n%s", first, second)
		}
	})

	t.Run("refuses bad JSON and writes nothing", func(t *testing.T) {
		worktree := t.TempDir()
		path := filepath.Join(worktree, relPath)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		bad := []byte("{not json")
		if err := os.WriteFile(path, bad, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := install.Install(worktree, host); err == nil {
			t.Fatal("Install accepted bad JSON")
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, bad) {
			t.Fatalf("file changed to %q", got)
		}
	})
}

func TestInstallClaude(t *testing.T) {
	checkInstall(t, shareddomain.HostClaude, filepath.Join(".claude", "settings.json"))
}

func TestInstallCodex(t *testing.T) {
	checkInstall(t, shareddomain.HostCodex, filepath.Join(".codex", "hooks.json"))
}
