package persistence_test

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/host/domain"
	"github.com/90p-labs/harness/internal/host/infrastructure/persistence"
	"github.com/90p-labs/harness/internal/shared/infrastructure/dbal"
)

const (
	runID      = "20261110-011312-trace"
	otherRunID = "20261110-020000-other"
	markID     = "turn-1"
)

var first = time.Date(2026, 11, 10, 1, 0, 0, 0, time.FixedZone("CET", 3600))

func openMarks(t *testing.T) persistence.SQLAgentMarks {
	t.Helper()
	ctx := context.Background()
	db, err := dbal.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "state.db"), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(ctx, persistence.Module, persistence.Migrations); err != nil {
		t.Fatal(err)
	}
	return persistence.NewSQLAgentMarks(db)
}

func mustOpen(t *testing.T, m persistence.SQLAgentMarks, run string, kind domain.MarkKind, at time.Time) {
	t.Helper()
	if err := m.Open(context.Background(), run, kind, markID, at); err != nil {
		t.Fatalf("Open: %v", err)
	}
}

func assertClose(t *testing.T, m persistence.SQLAgentMarks, run string, kind domain.MarkKind, wantAt time.Time, wantOK bool) {
	t.Helper()
	at, ok, err := m.Close(context.Background(), run, kind, markID)
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
	if ok != wantOK || !at.Equal(wantAt) {
		t.Fatalf("Close = (%v, %v), want (%v, %v)", at, ok, wantAt, wantOK)
	}
}

func TestOpenKeepsFirstTimeAndCloseReturnsIt(t *testing.T) {
	m := openMarks(t)
	mustOpen(t, m, runID, domain.MarkTurn, first)
	mustOpen(t, m, runID, domain.MarkTurn, first.Add(time.Hour))

	assertClose(t, m, runID, domain.MarkTurn, first, true)
	assertClose(t, m, runID, domain.MarkTurn, time.Time{}, false)
}

func TestMarksOfOtherRunOrKindAreSeparate(t *testing.T) {
	m := openMarks(t)
	later := first.Add(time.Minute)
	mustOpen(t, m, runID, domain.MarkTurn, first)
	mustOpen(t, m, otherRunID, domain.MarkTurn, later)
	mustOpen(t, m, runID, domain.MarkSubagent, later.Add(time.Minute))

	assertClose(t, m, runID, domain.MarkTurn, first, true)
	assertClose(t, m, otherRunID, domain.MarkTurn, later, true)
	assertClose(t, m, runID, domain.MarkSubagent, later.Add(time.Minute), true)
}

func TestConcurrentClosesSucceedOnce(t *testing.T) {
	m := openMarks(t)
	mustOpen(t, m, runID, domain.MarkTurn, first)

	var closed atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, ok, err := m.Close(context.Background(), runID, domain.MarkTurn, markID); err == nil && ok {
				closed.Add(1)
			}
		}()
	}
	wg.Wait()
	if got := closed.Load(); got != 1 {
		t.Fatalf("%d closes succeeded, want 1", got)
	}
}
