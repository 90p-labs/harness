package recordapproval_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/run/application/command/recordapproval"
	"github.com/90p-labs/harness/internal/run/domain"
)

const (
	runID    = "20261108-170222-add-a-grill-step"
	planHash = "0fcc61abbe1c"
)

var now = time.Date(2026, 11, 8, 17, 2, 22, 0, time.UTC)

type fakeRepository struct {
	domain.RunRepository
	stored domain.Run
}

func (f fakeRepository) Get(context.Context, string) (domain.Run, error) { return f.stored, nil }

type fakeGate struct{ domain.PlanGate }

func (fakeGate) Hash(domain.Run) (string, error) { return planHash, nil }

type saved struct {
	runID, hash string
	at          time.Time
}

type fakeApprovals struct {
	domain.ApprovalRepository
	saved []saved
}

func (f *fakeApprovals) Save(_ context.Context, id, hash string, at time.Time) error {
	f.saved = append(f.saved, saved{id, hash, at})
	return nil
}

func TestApprovalRecordHandler(t *testing.T) {
	cases := []struct {
		name    string
		phase   domain.Phase
		wantErr error
		want    []saved
	}{
		{"awaiting approval records the plan hash", domain.PhaseAwaitingApproval, nil, []saved{{runID, planHash, now}}},
		{"another phase records nothing", domain.PhaseWorking, domain.ErrIllegalTransition, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			approvals := &fakeApprovals{}
			repo := fakeRepository{stored: domain.Run{ID: runID, Phase: tc.phase}}
			handler := recordapproval.NewHandler(repo, fakeGate{}, approvals, func() time.Time { return now })

			err := handler.Handle(context.Background(), recordapproval.Command{RunID: runID})

			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if len(approvals.saved) != len(tc.want) || (len(tc.want) > 0 && approvals.saved[0] != tc.want[0]) {
				t.Fatalf("saved = %+v, want %+v", approvals.saved, tc.want)
			}
		})
	}
}
