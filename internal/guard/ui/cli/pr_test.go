package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/guard/domain"
)

func TestPRCheck(t *testing.T) {
	const good = "Bottom line: x.\n\n## Why\nBecause.\n\n## What\n- one\n\n## How\nShort.\n"
	bad := "Bottom line: x.\n\n## Why\n" + strings.TrimSpace(strings.Repeat("word ", 41)) + "\n\n## What\n- one\n"

	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		cmd := NewPRCommand(fakeSource{})
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

type fakeSource struct {
	plan    string
	commits []string
	base    *string
}

func (f fakeSource) Plan(context.Context) (string, error) { return f.plan, nil }
func (f fakeSource) Commits(_ context.Context, base string) ([]string, error) {
	if f.base != nil {
		*f.base = base
	}
	return f.commits, nil
}

func TestPRBodyBaseFlag(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{"no flag keeps the default branch", []string{commandBody}, ""},
		{"flag names the base", []string{commandBody, "--base", "release"}, "release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := "unset"
			cmd := NewPRCommand(fakeSource{base: &got})
			cmd.SetArgs(tc.args)
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("base = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPRBodyPrintsSkeleton(t *testing.T) {
	plan := "# Plan\n\n## Tech specs\n\n### Flow\n\n```mermaid\nsequenceDiagram\n  a->>b: call\n```\n\nText.\n\n```go\nnot a diagram\n```\n"
	src := fakeSource{plan: plan, commits: []string{"feat(pr): print the PR body skeleton", "docs(skill): fill the body"}}
	var out bytes.Buffer
	cmd := NewPRCommand(src)
	cmd.SetArgs([]string{commandBody})
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	body := out.String()

	last := -1
	for _, want := range []string{bottomLinePlaceholder, "## Why", whyPlaceholder, "## What", "- feat(pr): print the PR body skeleton", "- docs(skill): fill the body", "## How", "```mermaid\nsequenceDiagram\n  a->>b: call\n```"} {
		at := strings.Index(body, want)
		if at < 0 || at < last {
			t.Fatalf("%q is missing or out of order in:\n%s", want, body)
		}
		last = at
	}
	if strings.Contains(body, "not a diagram") {
		t.Errorf("the body holds a fence that is not mermaid:\n%s", body)
	}
	filled := strings.NewReplacer(bottomLinePlaceholder, "The command prints the body.", whyPlaceholder, "The lead wrote it by hand.").Replace(body)
	if problems := domain.BodyProblems(filled); len(problems) != 0 {
		t.Errorf("problems = %v in:\n%s", problems, filled)
	}

	t.Run("no diagram", func(t *testing.T) {
		var out bytes.Buffer
		cmd := NewPRCommand(fakeSource{commits: []string{"fix: x"}})
		cmd.SetArgs([]string{commandBody})
		cmd.SetOut(&out)
		cmd.SetErr(io.Discard)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		filled := strings.NewReplacer(bottomLinePlaceholder, "Bottom.", whyPlaceholder, "Because.").Replace(out.String())
		if problems := domain.BodyProblems(filled); len(problems) != 0 {
			t.Errorf("problems = %v in:\n%s", problems, filled)
		}
	})
}
