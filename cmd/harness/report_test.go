package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	reportdomain "github.com/jorge07/harness/internal/report/domain"
)

const noRunLine = "- run: " + reportdomain.RunNone + "\n"

const (
	commandReport = "report"
	issueURL      = "https://github.com/jorge07/harness/issues/31"
	reportTitle   = "guard: commits-checked denies a push for dev@example.com"
	reportDraft   = `## Summary

The guard rule commits-checked denies git push after harness verify --commits passed for HEAD.

## Reproduction

### Commands

` + "```sh" + `
git init demo && cd demo
git commit --allow-empty -m "feat: start"
harness verify --commits HEAD~0
echo '{"tool_name":"Bash","tool_input":{"command":"git push"}}' | harness hook --host claude pre-tool-use
` + "```" + `

### Config

` + "```yaml" + `
gates:
  - id: go-test
    run: go test ./...
` + "```" + `

### Expected

The hook prints nothing and exits 0.

### Actual

The hook prints a deny for commits-checked. The log names /Users/dev/work/demo and GITHUB_TOKEN=ghp_aBcDeFgHiJkLmNoPqRsTuVwXyZ0123456789.
The scratch folder is /private/tmp/claude-501/-Users-dev-work-demo.

## Traces

### Host

Claude Code 2.1.3

### Output

` + "```text" + `
{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny"}}
` + "```" + `

## Fix suggestion

internal/guard/domain/rules_git.go, the commits-checked rule: compare the checked SHA with the full HEAD SHA.

## Workaround

Run harness verify --commits origin/main before the push.
`
	fakeGHScript = `#!/bin/sh
printf '%s\n' "$@" > "$(dirname "$0")/args.log"
cat > "$(dirname "$0")/stdin.log"
echo "$FAKE_GH_OUT"
echo "$FAKE_GH_ERR" >&2
exit "$FAKE_GH_EXIT"
`
)

func fakeGHOnPath(t *testing.T, out, stderr, exit string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte(fakeGHScript), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("FAKE_GH_OUT", out)
	t.Setenv("FAKE_GH_ERR", stderr)
	t.Setenv("FAKE_GH_EXIT", exit)
	return dir
}

func writeDraft(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "draft.md")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ghCalled(t *testing.T, dir string) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, "args.log"))
	return err == nil
}

func readGHLog(t *testing.T, dir, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func assertRedacted(t *testing.T, text string) {
	t.Helper()
	for _, leak := range []string{"dev@example.com", "/Users/dev", "ghp_"} {
		if strings.Contains(text, leak) {
			t.Fatalf("text leaks %q:\n%s", leak, text)
		}
	}
}

func TestReportCommand(t *testing.T) {
	t.Run("dry run", func(t *testing.T) {
		clearHarnessEnv(t)
		initRepo(t)
		gh := fakeGHOnPath(t, issueURL, "", "0")
		if code, _, stderr := execute(t, "run", "start", "Fix the push guard"); code != exitOK {
			t.Fatalf("run start: code = %d, stderr = %q", code, stderr)
		}

		code, stdout, stderr := execute(t, commandReport, "--dry-run", "--title", reportTitle, "--body-file", writeDraft(t, reportDraft))

		if code != exitOK {
			t.Fatalf("code = %d, stderr = %q", code, stderr)
		}
		for _, want := range []string{
			"guard: commits-checked denies a push for <redacted>\n\n## Summary\n",
			"## Traces\n\n### Environment\n\n- harness: " + testBuildVersion + "\n",
			", phase planning\n\n### Host\n",
		} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("stdout misses %q:\n%s", want, stdout)
			}
		}
		assertRedacted(t, stdout)
		if ghCalled(t, gh) {
			t.Fatal("a dry run called gh")
		}
	})

	t.Run("outside a repository", func(t *testing.T) {
		clearHarnessEnv(t)
		t.Chdir(t.TempDir())
		gh := fakeGHOnPath(t, issueURL, "", "0")

		code, stdout, stderr := execute(t, commandReport, "--dry-run", "--title", reportTitle, "--body-file", writeDraft(t, reportDraft))

		if code != exitOK || !strings.Contains(stdout, noRunLine) {
			t.Fatalf("code = %d, stderr = %q, want 0 and the run line none:\n%s", code, stderr, stdout)
		}
		if ghCalled(t, gh) {
			t.Fatal("a dry run called gh")
		}
	})

	t.Run("missing section", func(t *testing.T) {
		clearHarnessEnv(t)
		initRepo(t)
		gh := fakeGHOnPath(t, issueURL, "", "0")
		draft := strings.Replace(reportDraft, "### Expected\n\nThe hook prints nothing and exits 0.\n", "", 1)
		draft = strings.Replace(draft, "Run harness verify --commits origin/main before the push.", "[fill: the workaround]", 1)

		code, _, stderr := execute(t, commandReport, "--title", reportTitle, "--body-file", writeDraft(t, draft))

		if code != exitUsage || !strings.Contains(stderr, "Reproduction > Expected, Workaround\n") {
			t.Fatalf("code = %d, stderr = %q, want %d and the missing sections", code, stderr, exitUsage)
		}
		if ghCalled(t, gh) {
			t.Fatal("an incomplete draft called gh")
		}
	})

	t.Run("files", func(t *testing.T) {
		clearHarnessEnv(t)
		initRepo(t)
		gh := fakeGHOnPath(t, issueURL, "", "0")
		draft := writeDraft(t, reportDraft)
		t.Setenv("HOME", "/Users/dev")

		code, stdout, stderr := execute(t, commandReport, "--title", reportTitle, "--body-file", draft)

		if code != exitOK || stdout != issueURL+"\n" {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want 0 and the issue URL", code, stdout, stderr)
		}
		if args := readGHLog(t, gh, "args.log"); !strings.HasPrefix(args, "issue\ncreate\n-R\njorge07/harness\n--title\n") {
			t.Fatalf("gh args = %q", args)
		}
		body := readGHLog(t, gh, "stdin.log")
		assertRedacted(t, body)
		for _, want := range []string{"~/work/demo", "claude-501/~-work-demo.", noRunLine} {
			if !strings.Contains(body, want) {
				t.Fatalf("filed body misses %q:\n%s", want, body)
			}
		}
	})

	t.Run("gh fails", func(t *testing.T) {
		clearHarnessEnv(t)
		initRepo(t)
		fakeGHOnPath(t, "", "HTTP 401: Bad credentials", "1")

		code, stdout, stderr := execute(t, commandReport, "--title", reportTitle, "--body-file", writeDraft(t, reportDraft))

		if code != exitRuntime || stdout != "" || !strings.Contains(stderr, "HTTP 401: Bad credentials") {
			t.Fatalf("code = %d, stdout = %q, stderr = %q, want %d and the gh stderr", code, stdout, stderr, exitRuntime)
		}
	})
}
