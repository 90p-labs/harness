package persistence_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/gate/domain"
	"github.com/90p-labs/harness/internal/gate/infrastructure/persistence"
)

func TestReviewRecord(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewSQLReviewRecordRepository(openDB(t))
	const (
		otherHead = "d4e5f6a"
		patchID   = "p1"
		runID     = "run-1"
	)
	files := []string{"a.go", "dir/b.go"}
	save := func(record domain.ReviewRecord) {
		t.Helper()
		record.Repo = repoName
		if err := repo.Save(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	models := func(scope domain.ReviewScope) []string {
		t.Helper()
		scope.Repo = repoName
		records, err := repo.Matching(ctx, scope)
		if err != nil {
			t.Fatal(err)
		}
		var found []string
		for _, record := range records {
			found = append(found, record.Model)
		}
		return found
	}

	save(domain.ReviewRecord{HeadSHA: headSHA, Model: "sonnet", FilesRead: files, PatchID: patchID, RunID: runID, At: clock})
	save(domain.ReviewRecord{HeadSHA: otherHead, Model: "haiku", RunID: runID, At: clock.Add(time.Minute)})
	save(domain.ReviewRecord{HeadSHA: "cafe", Model: "opus", FilesRead: files, At: clock})

	t.Run("head", func(t *testing.T) {
		if got := models(domain.ReviewScope{HeadSHA: headSHA}); !reflect.DeepEqual(got, []string{"sonnet"}) {
			t.Errorf("models = %v, want [sonnet]", got)
		}
	})
	t.Run("patch id of another head", func(t *testing.T) {
		if got := models(domain.ReviewScope{HeadSHA: "new", PatchID: patchID}); !reflect.DeepEqual(got, []string{"sonnet"}) {
			t.Errorf("models = %v, want [sonnet]", got)
		}
	})
	t.Run("empty patch id never matches", func(t *testing.T) {
		if got := models(domain.ReviewScope{HeadSHA: "new"}); len(got) != 0 {
			t.Errorf("models = %v, want none", got)
		}
	})
	t.Run("run match needs a run id and a later record", func(t *testing.T) {
		if got := models(domain.ReviewScope{HeadSHA: "new", RunID: runID, RunSince: clock.Add(30 * time.Second)}); !reflect.DeepEqual(got, []string{"haiku"}) {
			t.Errorf("after the rework: models = %v, want [haiku]", got)
		}
		if got := models(domain.ReviewScope{HeadSHA: "new", RunID: runID}); len(got) != 2 {
			t.Errorf("no rework: models = %v, want 2", got)
		}
		if got := models(domain.ReviewScope{HeadSHA: "new", RunSince: clock.Add(-time.Hour)}); len(got) != 0 {
			t.Errorf("strict: models = %v, want none", got)
		}
	})

	t.Run("files read round trip and keep", func(t *testing.T) {
		scope := domain.ReviewScope{Repo: repoName, HeadSHA: headSHA}
		save(domain.ReviewRecord{HeadSHA: headSHA, Model: "sonnet", RunID: runID, At: clock.Add(time.Hour)})
		records, err := repo.Matching(ctx, scope)
		if err != nil || len(records) != 1 || !reflect.DeepEqual(records[0].FilesRead, files) || records[0].PatchID != patchID {
			t.Fatalf("after an empty save: %+v, %v, want the files and the patch id kept", records, err)
		}
		save(domain.ReviewRecord{HeadSHA: headSHA, Model: "sonnet", FilesRead: []string{"c.go"}, PatchID: "p2", RunID: runID, At: clock.Add(time.Hour)})
		records, err = repo.Matching(ctx, scope)
		if err != nil || len(records) != 1 || !reflect.DeepEqual(records[0].FilesRead, []string{"c.go"}) || records[0].PatchID != "p2" {
			t.Fatalf("after a save with files: %+v, %v, want c.go and p2", records, err)
		}
	})
}
