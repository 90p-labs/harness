package persistence_test

import (
	"context"
	"testing"

	"github.com/90p-labs/harness/internal/gate/infrastructure/persistence"
)

const stepID = "deps"

func TestSetupRepository(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewSQLSetupRepository(openDB(t))

	if _, ok, err := repo.LastPass(ctx, repoName, stepID); err != nil || ok {
		t.Fatalf("empty store: ok = %v, err = %v", ok, err)
	}
	for _, hash := range []string{"lock-a", "lock-b"} {
		if err := repo.SavePass(ctx, repoName, stepID, hash, clock); err != nil {
			t.Fatal(err)
		}
	}
	hash, ok, err := repo.LastPass(ctx, repoName, stepID)
	if err != nil || !ok || hash != "lock-b" {
		t.Errorf("last pass = %q, %v, %v, want the newest hash lock-b", hash, ok, err)
	}
	if _, ok, _ := repo.LastPass(ctx, "other/repo", stepID); ok {
		t.Error("another repo sees the pass")
	}
}
