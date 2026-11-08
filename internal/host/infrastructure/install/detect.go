package install

import (
	"os"
	"path/filepath"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const envClaudeSession = "CLAUDECODE"

// Codex exports these only inside its sandboxed shell, so a full-access Codex session falls back to the folder.
var envCodexSandbox = []string{"CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED"}

func DetectHosts(worktree string, getenv func(string) string) []shareddomain.Host {
	var fromEnv []shareddomain.Host
	if getenv(envClaudeSession) != "" {
		fromEnv = append(fromEnv, shareddomain.HostClaude)
	}
	for _, key := range envCodexSandbox {
		if getenv(key) != "" {
			fromEnv = append(fromEnv, shareddomain.HostCodex)
			break
		}
	}
	if len(fromEnv) > 0 {
		return fromEnv
	}
	var fromFolders []shareddomain.Host
	if isDir(filepath.Join(worktree, claudeSettingsDir)) {
		fromFolders = append(fromFolders, shareddomain.HostClaude)
	}
	if isDir(filepath.Join(worktree, codexSettingsDir)) {
		fromFolders = append(fromFolders, shareddomain.HostCodex)
	}
	return fromFolders
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
