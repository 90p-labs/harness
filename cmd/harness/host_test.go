package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	envClaudeSession   = "CLAUDECODE"
	envCodexSandbox    = "CODEX_SANDBOX"
	envCodexNoNetwork  = "CODEX_SANDBOX_NETWORK_DISABLED"
	codexFolder        = ".codex"
	codexHooksFile     = "hooks.json"
	commandHost        = "host"
	subcommandInstall  = "install"
	noHostError        = "error: no host found"
	manyHostsError     = "error: more than one host found"
	claudeFolder       = ".claude"
	claudeSettingsFile = "settings.json"
	claudeSessionOn    = "1"
)

func clearHostEnv(t *testing.T) {
	t.Helper()
	clearHarnessEnv(t)
	for _, name := range []string{envClaudeSession, envCodexSandbox, envCodexNoNetwork} {
		t.Setenv(name, "")
	}
}

func TestHostInstallDetectsTheHost(t *testing.T) {
	t.Run("the only host folder is the host", func(t *testing.T) {
		clearHostEnv(t)
		dir := initRepo(t)
		if err := os.Mkdir(filepath.Join(dir, codexFolder), 0o750); err != nil {
			t.Fatal(err)
		}

		code, stdout, stderr := execute(t, commandHost, subcommandInstall)

		want := filepath.Join(dir, codexFolder, codexHooksFile)
		if code != exitOK || strings.TrimSpace(stdout) != want {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0 and %q", code, stdout, stderr, want)
		}
	})

	t.Run("a Claude Code session beats both host folders", func(t *testing.T) {
		clearHostEnv(t)
		t.Setenv(envClaudeSession, claudeSessionOn)
		dir := initRepo(t)
		for _, folder := range []string{claudeFolder, codexFolder} {
			if err := os.Mkdir(filepath.Join(dir, folder), 0o750); err != nil {
				t.Fatal(err)
			}
		}

		code, stdout, stderr := execute(t, commandHost, subcommandInstall)

		want := filepath.Join(dir, claudeFolder, claudeSettingsFile)
		if code != exitOK || strings.TrimSpace(stdout) != want {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0 and %q", code, stdout, stderr, want)
		}
	})

	refusals := []struct {
		name      string
		folders   []string
		wantError string
	}{
		{name: "no host is a usage error", wantError: noHostError},
		{name: "two hosts is a usage error", folders: []string{claudeFolder, codexFolder}, wantError: manyHostsError},
	}
	for _, tc := range refusals {
		t.Run(tc.name, func(t *testing.T) {
			clearHostEnv(t)
			dir := initRepo(t)
			for _, folder := range tc.folders {
				if err := os.Mkdir(filepath.Join(dir, folder), 0o750); err != nil {
					t.Fatal(err)
				}
			}

			code, _, stderr := execute(t, commandHost, subcommandInstall)

			if code != exitUsage || !strings.HasPrefix(stderr, tc.wantError) {
				t.Fatalf("code = %d, stderr = %q, want %d and prefix %q", code, stderr, exitUsage, tc.wantError)
			}
		})
	}
}
