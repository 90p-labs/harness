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

	t.Run("the same review saved twice keeps one row", func(t *testing.T) {
		// Two SubagentStop events for one reviewer save the same head, model and patch id.
		const twiceHead = "beef"
		review := domain.ReviewRecord{HeadSHA: twiceHead, Model: "sonnet", FilesRead: files, PatchID: "p-twice", RunID: runID, At: clock}
		save(review)
		review.At = clock.Add(time.Second)
		save(review)
		if got := models(domain.ReviewScope{HeadSHA: twiceHead}); !reflect.DeepEqual(got, []string{"sonnet"}) {
			t.Errorf("models = %v, want one sonnet row", got)
		}
		if got := models(domain.ReviewScope{HeadSHA: "new", PatchID: "p-twice"}); !reflect.DeepEqual(got, []string{"sonnet"}) {
			t.Errorf("by patch id: models = %v, want one sonnet row", got)
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

func TestLastReviewedByAll(t *testing.T) {
	ctx := context.Background()
	repo := persistence.NewSQLReviewRecordRepository(openDB(t))
	files := []string{"a.go"}
	save := func(head, model, run string, read []string, at time.Time) {
		t.Helper()
		if err := repo.Save(ctx, domain.ReviewRecord{Repo: repoName, HeadSHA: head, Model: model, RunID: run, FilesRead: read, At: at}); err != nil {
			t.Fatal(err)
		}
	}
	both := []string{"haiku", "sonnet"}
	last := func(run string, models []string) string {
		t.Helper()
		got, err := repo.LastReviewedByAll(ctx, repoName, run, models)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if got := last("run-1", both); got != "" {
		t.Fatalf("no records: head = %q, want none", got)
	}
	save("h1", "haiku", "run-1", files, clock)
	save("h1", "sonnet", "run-1", files, clock)
	save("h2", "haiku", "run-1", files, clock.Add(time.Minute))
	save("h2", "sonnet", "run-1", nil, clock.Add(time.Minute))
	save("h3", "haiku", "run-2", files, clock.Add(2*time.Minute))
	save("h3", "sonnet", "run-2", files, clock.Add(2*time.Minute))
	if got := last("run-1", both); got != "h1" {
		t.Errorf("head = %q, want h1 (h2 has an unread record)", got)
	}
	save("h4", "sonnet", "run-1", files, clock.Add(3*time.Minute))
	save("h4", "haiku", "run-1", files, clock.Add(4*time.Minute))
	if got := last("run-1", both); got != "h4" {
		t.Errorf("head = %q, want the latest head h4", got)
	}
	if got := last("run-1", []string{"haiku", "opus"}); got != "" {
		t.Errorf("head = %q, want none for a model that never reviewed", got)
	}
	if got := last("", both); got != "" {
		t.Errorf("head = %q, want none without a run", got)
	}
	if got := last("run-1", nil); got != "" {
		t.Errorf("head = %q, want none without models", got)
	}
}
