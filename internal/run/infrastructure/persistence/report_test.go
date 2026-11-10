package persistence_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/run/domain"
)

const reasonsFile = "testdata/rework-reasons.jsonl"

type reasonRow struct {
	Run   string             `json:"run"`
	Class domain.ReworkClass `json:"class"`
}

func readReasons(t *testing.T) []reasonRow {
	t.Helper()
	file, err := os.Open(reasonsFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var rows []reasonRow
	scanner := bufio.NewScanner(file)
	scanner.Buffer(nil, 1<<20)
	for scanner.Scan() {
		var row reasonRow
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

func seedReworks(t *testing.T, s store, repo string, rows []reasonRow) {
	t.Helper()
	runs := map[string]bool{}
	for _, row := range rows {
		if !runs[row.Run] {
			runs[row.Run] = true
			run := newRun(t, row.Run, domain.EventStart)
			run.Key = domain.RunKey{Repo: repo, Worktree: "/" + row.Run}
			run.Phase = domain.PhaseDone
			save(t, s, run)
		}
		_, err := s.db.Exec("INSERT INTO run_transitions (run_id, at, from_phase, event, to_phase, reason, rework_class) VALUES (?, ?, ?, ?, ?, ?, ?)",
			row.Run, clock.UTC(), domain.PhaseReviewing, domain.EventRequestRework, domain.PhaseWorking, "reason", row.Class)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRepeatClassesCount(t *testing.T) {
	ctx := context.Background()
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("the audit reasons give the repeat classes in order", func(t *testing.T) {
				s := openStore(t, c)
				seedReworks(t, s, key.Repo, readReasons(t))

				got, err := s.repo.RepeatClasses(ctx, key.Repo)

				want := []domain.ClassCount{
					{Class: domain.ReworkImplementation, Count: 32},
					{Class: domain.ReworkMinor, Count: 7},
					{Class: domain.ReworkSpecGap, Count: 6},
					{Class: domain.ReworkGateCheckable, Count: 5},
				}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("got %+v, %v, want %+v", got, err, want)
				}
			})

			t.Run("a class with one occurrence is absent and another repository does not count", func(t *testing.T) {
				s := openStore(t, c)
				rows := []reasonRow{
					{Run: "a", Class: domain.ReworkMinor}, {Run: "a", Class: domain.ReworkMinor},
					{Run: "b", Class: domain.ReworkSpecGap},
				}
				seedReworks(t, s, key.Repo, rows)
				seedReworks(t, s, "github.com/other/repo", []reasonRow{{Run: "c", Class: domain.ReworkSpecGap}})

				got, err := s.repo.RepeatClasses(ctx, key.Repo)

				want := []domain.ClassCount{{Class: domain.ReworkMinor, Count: 2}}
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("got %+v, %v, want %+v", got, err, want)
				}
			})
		})
	}
}

func TestReportTransitionsOfARun(t *testing.T) {
	ctx := context.Background()
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			s := openStore(t, c)
			run := newRun(t, "r1", domain.EventStart)
			save(t, s, run)
			other := newRun(t, "r2", domain.EventStart)
			other.Key.Worktree = "/other"
			save(t, s, other)
			second := clock.Add(time.Minute).UTC()
			for _, row := range []struct {
				at    time.Time
				event domain.Event
				class domain.ReworkClass
			}{
				{second, domain.EventRequestRework, domain.ReworkMinor},
				{clock.UTC(), domain.EventSubmitPlan, ""},
			} {
				if _, err := s.db.Exec("INSERT INTO run_transitions (run_id, at, from_phase, event, to_phase, reason, rework_class) VALUES (?, ?, ?, ?, ?, ?, ?)",
					"r1", row.at, domain.PhasePlanning, row.event, domain.PhaseWorking, "", row.class); err != nil {
					t.Fatal(err)
				}
			}

			got, err := s.repo.Transitions(ctx, "r1")

			if err != nil || len(got) != 3 {
				t.Fatalf("got %+v, %v", got, err)
			}
			if got[0].Event != domain.EventStart || got[1].Event != domain.EventSubmitPlan || got[2].Event != domain.EventRequestRework {
				t.Fatalf("events = %v, %v, %v, want order by time", got[0].Event, got[1].Event, got[2].Event)
			}
			if got[2].Class != domain.ReworkMinor || !got[2].At.Equal(second) || got[2].RunID != "r1" || got[2].To != domain.PhaseWorking {
				t.Fatalf("row = %+v", got[2])
			}
		})
	}
}
