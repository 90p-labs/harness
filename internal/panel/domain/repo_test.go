package domain_test

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/90p-labs/harness/internal/panel/domain"
)

func TestRepoKey(t *testing.T) {
	sum := sha256.Sum256([]byte("/work/a/.git"))
	want := hex.EncodeToString(sum[:])[:12]

	got := domain.RepoKey("/work/a/.git")
	if got != want {
		t.Fatalf("RepoKey = %q, want %q", got, want)
	}
	if other := domain.RepoKey("/work/b/.git"); other == got {
		t.Errorf("two common dirs share the key %q", got)
	}
	if again, err := domain.ParseRepoKey(got); err != nil || again != got {
		t.Errorf("ParseRepoKey(%q) = %q, %v", got, again, err)
	}

	for _, bad := range []string{"", "abc", "0123456789a", "0123456789abc", "0123456789AB", "0123456789gh", "../../etc/pa", "0123456789a/"} {
		if _, err := domain.ParseRepoKey(bad); err == nil {
			t.Errorf("ParseRepoKey(%q) accepted a bad key", bad)
		}
	}
}
