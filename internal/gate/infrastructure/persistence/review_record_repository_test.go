package persistence_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/gate/infrastructure/persistence"
)

func TestReviewRecord(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewSQLReviewRecordRepository(openDB(t))
	otherHead := "d4e5f6a"

	for _, model := range []string{"sonnet", "sonnet", "haiku"} {
		if err := repo.Save(ctx, domain.ReviewRecord{Repo: repoName, HeadSHA: headSHA, Model: model, At: clock}); err != nil {
			t.Fatal(err)
		}
	}

	models, err := repo.Models(ctx, repoName, headSHA)
	if err != nil || !reflect.DeepEqual(models, []string{"haiku", "sonnet"}) {
		t.Errorf("models = %v, %v, want [haiku sonnet]", models, err)
	}
	models, err = repo.Models(ctx, repoName, otherHead)
	if err != nil || len(models) != 0 {
		t.Errorf("other head models = %v, %v, want none", models, err)
	}
}
