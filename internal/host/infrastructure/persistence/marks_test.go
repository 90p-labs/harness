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

func TestToolMarkRoundTrip(t *testing.T) {
	m := openMarks(t)
	ctx := context.Background()
	call := domain.ToolMark{ID: "call-1", SessionID: "s1", Tool: "Bash", StartedAt: first}
	other := domain.ToolMark{ID: "call-2", SessionID: "s2", Tool: "Read", StartedAt: first.Add(time.Minute)}
	foreign := domain.ToolMark{ID: "call-3", SessionID: "s1", Tool: "Edit", StartedAt: first}
	for run, mark := range map[string]domain.ToolMark{runID: call, otherRunID: foreign} {
		if err := m.OpenTool(ctx, run, mark); err != nil {
			t.Fatalf("OpenTool: %v", err)
		}
	}
	if err := m.OpenTool(ctx, runID, other); err != nil {
		t.Fatalf("OpenTool: %v", err)
	}

	got, err := m.CloseSession(ctx, runID, "s1")
	if err != nil {
		t.Fatalf("CloseSession: %v", err)
	}
	if len(got) != 1 || got[0].ID != call.ID || got[0].SessionID != call.SessionID || got[0].Tool != call.Tool || !got[0].StartedAt.Equal(call.StartedAt) {
		t.Fatalf("CloseSession = %+v, want [%+v]", got, call)
	}
	again, err := m.CloseSession(ctx, runID, "s1")
	if err != nil || len(again) != 0 {
		t.Fatalf("second CloseSession = %+v, %v; want none", again, err)
	}
	at, ok, err := m.Close(ctx, runID, domain.MarkTool, other.ID)
	if err != nil || !ok || !at.Equal(other.StartedAt) {
		t.Fatalf("Close = (%v, %v, %v), want (%v, true, nil)", at, ok, err, other.StartedAt)
	}
	if left, _ := m.CloseSession(ctx, otherRunID, "s1"); len(left) != 1 {
		t.Fatalf("the other run keeps %d marks, want 1", len(left))
	}
}
