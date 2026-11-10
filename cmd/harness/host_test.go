package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/shared/infrastructure/config"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	freshStopJSON     = `{"cwd":".","session_id":"s1"}`
	subagentStartJSON = `{"cwd":".","session_id":"s1","agent_id":"a1","agent_type":"Explore"}`
	commandSubagent   = "subagent-start"
	maxStopOpens      = 2
)

func countStateOpens(t *testing.T) *int {
	t.Helper()
	opens := 0
	previous := openStateDB
	openStateDB = func(ctx context.Context, cfg config.Config, commonDir string) (*dbal.DB, error) {
		opens++
		return previous(ctx, cfg, commonDir)
	}
	t.Cleanup(func() { openStateDB = previous })
	return &opens
}

func TestStopHookStateOpens(t *testing.T) {
	clearHarnessEnv(t)
	initRepo(t)
	execute(t, commandRun, "start", "Add the login page")
	opens := countStateOpens(t)

	for attempt := 1; attempt <= maxStopAttempts; attempt++ {
		*opens = 0
		code, stdout, stderr := executeWithStdin(t, freshStopJSON, commandHook, initHostFlag, hostClaude, "stop")
		if code != exitOK {
			t.Fatalf("stop: code = %d, stderr = %q", code, stderr)
		}
		if *opens > maxStopOpens {
			t.Fatalf("stop attempt %d: state opens = %d, want at most %d", attempt, *opens, maxStopOpens)
		}
		if stdout == "" {
			return
		}
	}
	t.Fatal("the guard held every stop")
}

func TestClosedRunDeletesMarks(t *testing.T) {
	clearHarnessEnv(t)
	dir := initRepo(t)
	execute(t, commandRun, "start", "Add the login page")
	if code, _, stderr := executeWithStdin(t, subagentStartJSON, commandHook, initHostFlag, hostClaude, commandSubagent); code != exitOK {
		t.Fatalf("subagent-start: code = %d, stderr = %q", code, stderr)
	}
	db, err := sql.Open("sqlite", defaultStatePath(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	countMarks := func() (marks int) {
		if err := db.QueryRowContext(context.Background(), "SELECT count(*) FROM agent_marks").Scan(&marks); err != nil {
			t.Fatal(err)
		}
		return marks
	}
	if countMarks() == 0 {
		t.Fatal("the subagent start opened no mark")
	}

	if code, _, stderr := execute(t, commandRun, "abandon"); code != exitOK {
		t.Fatalf("abandon: code = %d, stderr = %q", code, stderr)
	}

	if got := countMarks(); got != 0 {
		t.Fatalf("agent_marks rows = %d, want 0 after the run is abandoned", got)
	}
}

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
