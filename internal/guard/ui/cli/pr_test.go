package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPRCheck(t *testing.T) {
	const good = "Bottom line: x.\n\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort.\n"
	bad := "Bottom line: x.\n\n## Why\n" + strings.TrimSpace(strings.Repeat("word ", 41)) + "\n\n## What\n- one\n"

	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := NewPRCommand()
		cmd.SilenceUsage = true
		cmd.SetArgs(args)
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		err := cmd.Execute()
		return out.String(), err
	}
	file := func(body string) string {
		path := filepath.Join(t.TempDir(), "body.md")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}

	t.Run("good body", func(t *testing.T) {
		out, err := run(commandCheck, "--body-file", file(good))
		if err != nil || out != bodyOK+"\n" {
			t.Fatalf("out = %q, err = %v", out, err)
		}
	})
	t.Run("bad body", func(t *testing.T) {
		out, err := run(commandCheck, "--body-file", file(bad))
		if !errors.Is(err, errBodyProblems) {
			t.Fatalf("err = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 2 || !strings.Contains(lines[0], "the body has no ## How line") || !strings.Contains(lines[1], "line 4 has 41 words") {
			t.Fatalf("out = %q", out)
		}
	})
	t.Run("no body file", func(t *testing.T) {
		if _, err := run(commandCheck); err == nil || !strings.Contains(err.Error(), "--body-file is required") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("unreadable body file", func(t *testing.T) {
		if _, err := run(commandCheck, "--body-file", filepath.Join(t.TempDir(), "none.md")); err == nil {
			t.Fatal("no error")
		}
	})
}
