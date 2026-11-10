package persistence_test

import (
	"context"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/run/domain"
	"github.com/90p-labs/harness/internal/run/infrastructure/persistence"
)

const (
	approvedHash = "0fcc61abbe1c"
	editedHash   = "9d2e7a41c003"
)

func TestApprovalRecordRepository(t *testing.T) {
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			s := openStore(t, c)
			run := newRun(t, "20261108-170222-add-a-grill-step", domain.EventStart)
			if _, err := s.repo.Save(ctx, run); err != nil {
				t.Fatal(err)
			}
			approvals := persistence.NewSQLApprovalRepository(s.db)

			assertHas(t, approvals, run.ID, approvedHash, false)
			for range 2 {
				if err := approvals.Save(ctx, run.ID, approvedHash, clock); err != nil {
					t.Fatalf("Save: %v", err)
				}
			}
			assertHas(t, approvals, run.ID, approvedHash, true)
			assertHas(t, approvals, run.ID, editedHash, false)
		})
	}
}

func TestApprovalLatest(t *testing.T) {
	for _, c := range dialectCases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			s := openStore(t, c)
			run := newRun(t, "20261108-170222-add-a-grill-step", domain.EventStart)
			if _, err := s.repo.Save(ctx, run); err != nil {
				t.Fatal(err)
			}
			approvals := persistence.NewSQLApprovalRepository(s.db)
			save := func(hash string, at time.Time) {
				t.Helper()
				if err := approvals.Save(ctx, run.ID, hash, at); err != nil {
					t.Fatal(err)
				}
			}
			assertLatest := func(want string) {
				t.Helper()
				got, ok, err := approvals.Latest(ctx, run.ID)
				if err != nil || !ok || got != want {
					t.Fatalf("Latest = %q, %v, %v, want %q", got, ok, err, want)
				}
			}

			if _, ok, err := approvals.Latest(ctx, run.ID); err != nil || ok {
				t.Fatalf("Latest with no row = %v, %v", ok, err)
			}
			save(approvedHash, clock)
			save(editedHash, clock.Add(time.Minute))
			assertLatest(editedHash)
			save(approvedHash, clock.Add(2*time.Minute))
			assertLatest(approvedHash)
			save(editedHash, clock.Add(2*time.Minute))
			assertLatest(editedHash)
		})
	}
}

func assertHas(t *testing.T, approvals persistence.SQLApprovalRepository, runID, hash string, want bool) {
	t.Helper()
	got, err := approvals.Has(context.Background(), runID, hash)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Has(%s) = %v, want %v", hash, got, want)
	}
}
