package github_test

import (
	"context"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/shared/infrastructure/github"
)

const shell = "sh"

func TestRun(t *testing.T) {
	t.Run("passes stdin and returns stdout", func(t *testing.T) {
		out, err := github.Run(context.Background(), shell, strings.NewReader("issue body"), "-c", "cat")
		if err != nil {
			t.Fatal(err)
		}
		if string(out) != "issue body" {
			t.Fatalf("stdout = %q, want the stdin", out)
		}
	})

	t.Run("puts the trimmed stderr in the error", func(t *testing.T) {
		_, err := github.Run(context.Background(), shell, nil, "-c", "echo '  label not found  ' >&2; exit 1")
		if err == nil {
			t.Fatal("Run returned no error for exit 1")
		}
		if want := ": label not found"; !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("error = %q, want the suffix %q", err, want)
		}
		if !strings.HasPrefix(err.Error(), "gh -c ") {
			t.Fatalf("error = %q, want the gh args first", err)
		}
	})
}
