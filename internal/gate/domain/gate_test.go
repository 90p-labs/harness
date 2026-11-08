package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jorge07/harness/internal/gate/domain"
)

func TestNewGateValidation(t *testing.T) {
	cases := []struct {
		name      string
		id        string
		run       string
		timeout   time.Duration
		wantErr   bool
		wantNamed string
		wantTime  time.Duration
	}{
		{name: "empty id", id: "", run: "go test ./...", wantErr: true, wantNamed: `gate ""`},
		{name: "uppercase id", id: "Go-Test", run: "go test ./...", wantErr: true, wantNamed: "Go-Test"},
		{name: "id starting with a digit", id: "1test", run: "go test ./...", wantErr: true, wantNamed: "1test"},
		{name: "id with underscore", id: "go_test", run: "go test ./...", wantErr: true, wantNamed: "go_test"},
		{name: "empty run", id: "go-test", run: "", wantErr: true, wantNamed: "go-test"},
		{name: "negative timeout", id: "go-test", run: "go test ./...", timeout: -time.Second, wantErr: true, wantNamed: "go-test"},
		{name: "default timeout", id: "go-test", run: "go test ./...", wantTime: domain.DefaultTimeout},
		{name: "explicit timeout", id: "criterion-3", run: "true", timeout: 90 * time.Second, wantTime: 90 * time.Second},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := domain.NewGate(tc.id, tc.run, []string{"**/*.go"}, tc.timeout, "")
			if tc.wantErr {
				if !errors.Is(err, domain.ErrInvalidGate) {
					t.Fatalf("error = %v, want ErrInvalidGate", err)
				}
				if !strings.Contains(err.Error(), tc.wantNamed) {
					t.Fatalf("error %q does not name the gate %q", err, tc.wantNamed)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewGate error = %v", err)
			}
			if got.ID != tc.id || got.Run != tc.run || got.Timeout != tc.wantTime || len(got.Paths) != 1 {
				t.Fatalf("gate = %+v", got)
			}
		})
	}
}

func TestNewGateValidationRejectsDuplicateID(t *testing.T) {
	first := domain.Gate{ID: "go-test", Run: "go test ./..."}
	other := domain.Gate{ID: "go-vet", Run: "go vet ./..."}

	if err := domain.ValidateGates([]domain.Gate{first, other}); err != nil {
		t.Fatalf("distinct ids: %v", err)
	}

	err := domain.ValidateGates([]domain.Gate{first, other, first})
	if !errors.Is(err, domain.ErrInvalidGate) || !strings.Contains(err.Error(), "go-test") {
		t.Fatalf("duplicate id error = %v", err)
	}
}

func TestCriterionGateID(t *testing.T) {
	id := domain.CriterionGateID(12)
	if id != "criterion-12" {
		t.Fatalf("id = %q", id)
	}
	if !domain.IsCriterionGate(id) || domain.IsCriterionGate("go-test") {
		t.Fatal("IsCriterionGate does not tell the criterion gate")
	}
}
