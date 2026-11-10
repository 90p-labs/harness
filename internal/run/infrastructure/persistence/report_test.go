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
	seedEvents(t, s, repo, domain.EventRequestRework, rows)
}

func seedEvents(t *testing.T, s store, repo string, event domain.Event, rows []reasonRow) {
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
			row.Run, clock.UTC(), domain.PhaseReviewing, event, domain.PhaseWorking, "reason", row.Class)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLastReworkAt(t *testing.T) {
	ctx := context.Background()
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			s := openStore(t, c)
			save(t, s, newRun(t, "r1", domain.EventStart))
			other := newRun(t, "r2", domain.EventStart)
			other.Key.Worktree = "/other"
			save(t, s, other)

			if got, err := s.repo.LastReworkAt(ctx, "r1"); err != nil || !got.IsZero() {
				t.Fatalf("no rework: got %v, %v, want the zero time", got, err)
			}

			amend, rework := clock.Add(time.Minute).UTC(), clock.Add(3*time.Minute).UTC()
			for _, row := range []struct {
				run   string
				at    time.Time
				event domain.Event
			}{
				{"r1", amend, domain.EventAmend},
				{"r1", rework, domain.EventRequestRework},
				{"r1", clock.Add(5 * time.Minute).UTC(), domain.EventReworkMinor},
				{"r2", clock.Add(9 * time.Minute).UTC(), domain.EventRequestRework},
			} {
				if _, err := s.db.Exec("INSERT INTO run_transitions (run_id, at, from_phase, event, to_phase, reason, rework_class) VALUES (?, ?, ?, ?, ?, ?, ?)",
					row.run, row.at, domain.PhaseReviewing, row.event, domain.PhaseWorking, "", ""); err != nil {
					t.Fatal(err)
				}
			}

			got, err := s.repo.LastReworkAt(ctx, "r1")

			if err != nil || !got.Equal(rework) {
				t.Fatalf("got %v, %v, want %v, the last request-rework or amend of the run", got, err, rework)
			}
		})
	}
}

func TestReworkReasons(t *testing.T) {
	ctx := context.Background()
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			s := openStore(t, c)
			save(t, s, newRun(t, "r1", domain.EventStart))
			other := newRun(t, "r2", domain.EventStart)
			other.Key.Worktree = "/other"
			save(t, s, other)
			for i, row := range []struct {
				run    string
				event  domain.Event
				reason string
			}{
				{"r1", domain.EventRequestRework, "F1, major, fix the pong"},
				{"r1", domain.EventReworkMinor, "F2, minor, tidy"},
				{"r1", domain.EventRequestRework, "F3, blocker, fix the ping"},
				{"r2", domain.EventRequestRework, "F9, major, other run"},
				{"r1", domain.EventRegress, "new plan"},
				{"r1", domain.EventRequestRework, "F1, major, fix the new pong"},
				{"r1", domain.EventAmend, "amend"},
			} {
				if _, err := s.db.Exec("INSERT INTO run_transitions (run_id, at, from_phase, event, to_phase, reason, rework_class) VALUES (?, ?, ?, ?, ?, ?, ?)",
					row.run, clock.Add(time.Duration(i)*time.Minute).UTC(), domain.PhaseReviewing, row.event, domain.PhaseWorking, row.reason, ""); err != nil {
					t.Fatal(err)
				}
			}

			got, err := s.repo.ReworkReasons(ctx, "r1")

			want := []string{"F1, major, fix the pong", "F3, blocker, fix the ping", "F1, major, fix the new pong"}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %v, %v, want %v, every request-rework reason of the run, also before a regress", got, err, want)
			}
		})
	}
}

func TestLastReworkAtLastEvent(t *testing.T) {
	ctx := context.Background()
	for _, c := range dialectCases {
		for _, last := range []domain.Event{domain.EventAmend, domain.EventRegress} {
			t.Run(c.name+"/"+string(last), func(t *testing.T) {
				s := openStore(t, c)
				save(t, s, newRun(t, "r1", domain.EventStart))
				want := clock.Add(5 * time.Minute).UTC()
				for _, row := range []struct {
					at    time.Time
					event domain.Event
				}{
					{clock.Add(time.Minute).UTC(), domain.EventRequestRework},
					{want, last},
				} {
					if _, err := s.db.Exec("INSERT INTO run_transitions (run_id, at, from_phase, event, to_phase, reason, rework_class) VALUES (?, ?, ?, ?, ?, ?, ?)",
						"r1", row.at, domain.PhaseReviewing, row.event, domain.PhaseWorking, "", ""); err != nil {
						t.Fatal(err)
					}
				}

				got, err := s.repo.LastReworkAt(ctx, "r1")

				if err != nil || !got.Equal(want) {
					t.Fatalf("got %v, %v, want %v, the time of the last %s", got, err, want, last)
				}
			})
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

func TestRepeatClassesCountMinorReworks(t *testing.T) {
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			s := openStore(t, c)
			seedEvents(t, s, key.Repo, domain.EventRequestRework, []reasonRow{{Run: "a", Class: domain.ReworkMinor}})
			seedEvents(t, s, key.Repo, domain.EventReworkMinor, []reasonRow{{Run: "b", Class: domain.ReworkMinor}})

			got, err := s.repo.RepeatClasses(context.Background(), key.Repo)

			want := []domain.ClassCount{{Class: domain.ReworkMinor, Count: 2}}
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("got %+v, %v, want %+v", got, err, want)
			}
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

func TestLastReworkEvent(t *testing.T) {
	ctx := context.Background()
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			s := openStore(t, c)
			save(t, s, newRun(t, "r1", domain.EventStart))
			if event, reason, err := s.repo.LastReworkEvent(ctx, "r1"); err != nil || event != "" || reason != "" {
				t.Fatalf("no rework: got %q, %q, %v, want none", event, reason, err)
			}
			for i, row := range []struct {
				event  domain.Event
				reason string
			}{
				{domain.EventRequestRework, "fix F1"},
				{domain.EventApproveReview, "ignored"},
				{domain.EventReworkMinor, "polish"},
				{domain.EventAmend, "amend it"},
			} {
				if _, err := s.db.Exec("INSERT INTO run_transitions (run_id, at, from_phase, event, to_phase, reason, rework_class) VALUES (?, ?, ?, ?, ?, ?, ?)",
					"r1", clock.Add(time.Duration(i)*time.Minute).UTC(), domain.PhaseReviewing, row.event, domain.PhaseWorking, row.reason, ""); err != nil {
					t.Fatal(err)
				}
			}
			event, reason, err := s.repo.LastReworkEvent(ctx, "r1")
			if err != nil || event != domain.EventAmend || reason != "amend it" {
				t.Fatalf("got %q, %q, %v, want amend and its reason", event, reason, err)
			}
		})
	}
}
