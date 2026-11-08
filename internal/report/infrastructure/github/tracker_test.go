package github_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/report/infrastructure/github"
)

const fakeGH = `#!/bin/sh
printf '%s\n' "$@" > "$(dirname "$0")/args.log"
cat > "$(dirname "$0")/stdin.log"
echo "$FAKE_GH_OUT"
exit "$FAKE_GH_EXIT"
`

func installFake(t *testing.T, out, exit string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "gh")
	if err := os.WriteFile(bin, []byte(fakeGH), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FAKE_GH_OUT", out)
	t.Setenv("FAKE_GH_EXIT", exit)
	return bin
}

func readLog(t *testing.T, bin, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(bin), name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestTrackerCreate(t *testing.T) {
	t.Run("files the issue with the body on stdin", func(t *testing.T) {
		bin := installFake(t, "https://github.com/jorge07/harness/issues/7", "0")

		url, err := github.New(bin).Create(context.Background(), "hook exits 2", "## Summary\n\nbody\n")
		if err != nil {
			t.Fatal(err)
		}

		if url != "https://github.com/jorge07/harness/issues/7" {
			t.Fatalf("url = %q", url)
		}
		wantArgs := "issue\ncreate\n-R\njorge07/harness\n--title\nhook exits 2\n--label\nbug\n--body-file\n-\n"
		if got := readLog(t, bin, "args.log"); got != wantArgs {
			t.Fatalf("args = %q, want %q", got, wantArgs)
		}
		if got := readLog(t, bin, "stdin.log"); got != "## Summary\n\nbody\n" {
			t.Fatalf("stdin = %q, want the body", got)
		}
	})

	t.Run("a gh failure is an error", func(t *testing.T) {
		bin := installFake(t, "", "1")

		_, err := github.New(bin).Create(context.Background(), "hook exits 2", "body")

		if err == nil || !strings.Contains(err.Error(), "issue create") {
			t.Fatalf("err = %v, want the gh command in the error", err)
		}
	})
}
