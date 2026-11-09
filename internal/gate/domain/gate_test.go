package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/gate/domain"
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

func TestValidateGatesNeeds(t *testing.T) {
	gate := func(id string, needs ...string) domain.Gate {
		return domain.Gate{ID: id, Run: "true", Needs: needs}
	}
	cases := []struct {
		name      string
		gates     []domain.Gate
		wantNamed string
		wantText  string
	}{
		{name: "unknown need", gates: []domain.Gate{gate("lint"), gate("test", "lint", "typo")}, wantNamed: `gate "test"`, wantText: "typo"},
		{name: "self need", gates: []domain.Gate{gate("lint", "lint")}, wantNamed: `gate "lint"`, wantText: "itself"},
		{name: "cycle", gates: []domain.Gate{gate("fmt"), gate("lint", "e2e"), gate("test", "lint"), gate("e2e", "test")}, wantNamed: `gate "lint"`, wantText: "cycle"},
		{name: "criterion id", gates: []domain.Gate{gate(domain.CriterionGateID(1)), gate("e2e", domain.CriterionGateID(1))}, wantNamed: `gate "criterion-1"`, wantText: domain.CriterionGatePrefix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.ValidateGates(tc.gates)
			if !errors.Is(err, domain.ErrInvalidGate) || !strings.Contains(err.Error(), tc.wantNamed) || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("error = %v, want ErrInvalidGate naming %s and %q", err, tc.wantNamed, tc.wantText)
			}
		})
	}

	chain := []domain.Gate{gate("e2e", "test"), gate("fmt"), gate("lint"), gate("test", "fmt", "lint")}
	if err := domain.ValidateGates(chain); err != nil {
		t.Fatalf("a valid chain listed out of order: %v", err)
	}
}

func TestStatusPassed(t *testing.T) {
	passed := map[domain.Status]bool{
		domain.StatusPass: true, domain.StatusCached: true,
		domain.StatusFail: false, domain.StatusVetoed: false, domain.StatusSkipped: false,
	}
	for status, want := range passed {
		if got := status.Passed(); got != want {
			t.Errorf("%s.Passed() = %v, want %v", status, got, want)
		}
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
