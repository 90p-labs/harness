package install_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/90p-labs/harness/internal/host/infrastructure/install"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	envClaude        = "CLAUDECODE"
	envCodexSandbox  = "CODEX_SANDBOX"
	envCodexNoNet    = "CODEX_SANDBOX_NETWORK_DISABLED"
	claudeFolder     = ".claude"
	codexFolder      = ".codex"
	envSet           = "1"
	codexSandboxMode = "seatbelt"
)

func TestDetectHosts(t *testing.T) {
	claude := []shareddomain.Host{shareddomain.HostClaude}
	codex := []shareddomain.Host{shareddomain.HostCodex}
	both := []shareddomain.Host{shareddomain.HostClaude, shareddomain.HostCodex}
	tests := map[string]struct {
		env     map[string]string
		folders []string
		want    []shareddomain.Host
	}{
		"claude session":                   {env: map[string]string{envClaude: envSet}, want: claude},
		"codex sandbox":                    {env: map[string]string{envCodexSandbox: codexSandboxMode}, want: codex},
		"codex sandbox with no network":    {env: map[string]string{envCodexNoNet: envSet}, want: codex},
		"claude session beats the folders": {env: map[string]string{envClaude: envSet}, folders: []string{claudeFolder, codexFolder}, want: claude},
		"two sessions":                     {env: map[string]string{envClaude: envSet, envCodexSandbox: codexSandboxMode}, want: both},
		"claude folder":                    {folders: []string{claudeFolder}, want: claude},
		"codex folder":                     {folders: []string{codexFolder}, want: codex},
		"both folders":                     {folders: []string{claudeFolder, codexFolder}, want: both},
		"nothing":                          {},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			worktree := t.TempDir()
			for _, folder := range tt.folders {
				if err := os.Mkdir(filepath.Join(worktree, folder), 0o750); err != nil {
					t.Fatal(err)
				}
			}
			getenv := func(key string) string { return tt.env[key] }

			got := install.DetectHosts(worktree, getenv)

			if !slices.Equal(got, tt.want) {
				t.Fatalf("hosts = %v, want %v", got, tt.want)
			}
		})
	}
}
