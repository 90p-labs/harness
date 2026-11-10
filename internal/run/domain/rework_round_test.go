package domain_test

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	maxRounds     = 3
	severitiesLog = "testdata/rework-severities.jsonl"
	defaultRounds = 18
	strictRounds  = 50
)

func finding(id string, severity shareddomain.ReviewSeverity) shareddomain.ReviewFinding {
	return shareddomain.ReviewFinding{ID: id, Severity: severity, Verdict: shareddomain.FindingReal, Status: shareddomain.FindingOpen}
}

func rework(t *testing.T, run domain.Run, policy domain.Policy, findings ...shareddomain.ReviewFinding) domain.Run {
	t.Helper()
	ids := []string{"fix"}
	for _, f := range findings {
		ids = append(ids, f.ID)
	}
	reason := strings.Join(ids, " ")
	event, err := domain.ReworkEvent(findings, policy, reason)
	if err != nil {
		t.Fatal(err)
	}
	return apply(t, run, policy, event, reason)
}

func TestReworkNeedsFindings(t *testing.T) {
	notReal := finding("F1", shareddomain.SeverityBlocker)
	notReal.Verdict = shareddomain.FindingNotReal
	closed := finding("F2", shareddomain.SeverityMajor)
	closed.Status = shareddomain.FindingClosed

	for name, findings := range map[string][]shareddomain.ReviewFinding{
		"none":     nil,
		"not real": {notReal},
		"closed":   {closed},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := domain.ReworkEvent(findings, domain.Policy{}, "F1 F2"); !errors.Is(err, domain.ErrNoOpenFinding) {
				t.Fatalf("got %v, want ErrNoOpenFinding", err)
			}
		})
	}
}

func TestReworkNamesFindings(t *testing.T) {
	major := finding("F1", shareddomain.SeverityMajor)
	blocker := finding("F10", shareddomain.SeverityBlocker)
	minor := finding("F3", shareddomain.SeverityMinor)
	closed := finding("F4", shareddomain.SeverityBlocker)
	closed.Status = shareddomain.FindingClosed
	findings := []shareddomain.ReviewFinding{major, blocker, minor, closed}

	for name, tc := range map[string]struct {
		reason  string
		missing string
	}{
		"names every blocker and major":     {"fix F1, F10", ""},
		"a minor and a closed need no name": {"F1 F10", ""},
		"F1 does not cover F10":             {"fix F10", "F1"},
		"F10 does not cover F1":             {"fix F1", "F10"},
		"no name":                           {"fix", "F1, F10"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := domain.ReworkEvent(findings, domain.Policy{}, tc.reason)
			if tc.missing == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, domain.ErrReasonNamesNoFinding) || !strings.Contains(err.Error(), tc.missing) {
				t.Fatalf("got %v, want ErrReasonNamesNoFinding naming %q", err, tc.missing)
			}
		})
	}
}

func TestReworkCountsOnce(t *testing.T) {
	policy := domain.Policy{MaxReviewRounds: maxRounds}
	run := domain.Run{Phase: domain.PhaseReviewing}

	next := rework(t, run, policy,
		finding("F1", shareddomain.SeverityMinor), finding("F2", shareddomain.SeverityMajor),
		finding("F3", shareddomain.SeverityBlocker), finding("F4", shareddomain.SeverityMinor))

	if next.Phase != domain.PhaseWorking || next.ReviewRound != 1 {
		t.Fatalf("got phase %s, round %d", next.Phase, next.ReviewRound)
	}
}

func TestMinorReworkKeepsRound(t *testing.T) {
	policy := domain.Policy{MaxReviewRounds: maxRounds}
	for _, round := range []int{0, maxRounds - 1} {
		run := domain.Run{Phase: domain.PhaseReviewing, ReviewRound: round}

		next := rework(t, run, policy, finding("F1", shareddomain.SeverityMinor))

		if next.Phase != domain.PhaseWorking || next.ReviewRound != round || next.Reason != "fix F1" {
			t.Fatalf("round %d: got phase %s, round %d, reason %q", round, next.Phase, next.ReviewRound, next.Reason)
		}
	}
}

func TestStrictMinorRework(t *testing.T) {
	policy := domain.Policy{MaxReviewRounds: maxRounds, StrictMinor: true}

	next := rework(t, domain.Run{Phase: domain.PhaseReviewing}, policy, finding("F1", shareddomain.SeverityMinor))
	if next.Phase != domain.PhaseWorking || next.ReviewRound != 1 {
		t.Fatalf("got phase %s, round %d", next.Phase, next.ReviewRound)
	}

	last := rework(t, domain.Run{Phase: domain.PhaseReviewing, ReviewRound: maxRounds - 1}, policy, finding("F1", shareddomain.SeverityMinor))
	if last.Phase != domain.PhaseBlocked {
		t.Fatalf("got phase %s, want blocked", last.Phase)
	}
}

func TestMinorReworkStoresClass(t *testing.T) {
	run := domain.Run{Phase: domain.PhaseReviewing}
	next, err := run.Apply(domain.Command{Event: domain.EventReworkMinor, Class: domain.ReworkMinor}, domain.Policy{})
	if err != nil {
		t.Fatal(err)
	}
	if got := next.Events()[0].Class; got != domain.ReworkMinor {
		t.Fatalf("class = %q", got)
	}
}

func TestAmendKeepsReviewRound(t *testing.T) {
	policy := domain.Policy{ApprovalNeeded: true, MaxReviewRounds: maxRounds}
	run := domain.Run{Phase: domain.PhaseReviewing}

	run = rework(t, run, policy, finding("F1", shareddomain.SeverityMajor))
	run = apply(t, run, policy, domain.EventAmend, "")
	if run.Phase != domain.PhaseAwaitingApproval {
		t.Fatalf("got phase %s", run.Phase)
	}
	run = apply(t, run, policy, domain.EventApprovePlan, "")

	if run.Phase != domain.PhaseWorking || run.ReviewRound != 1 {
		t.Fatalf("got phase %s, round %d, want working, 1", run.Phase, run.ReviewRound)
	}
}

func TestReplayReworkSeverities(t *testing.T) {
	file, err := os.Open(severitiesLog)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	var rows []struct {
		Severity shareddomain.ReviewSeverity `json:"severity"`
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var row struct {
			Severity shareddomain.ReviewSeverity `json:"severity"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}

	for name, tc := range map[string]struct {
		strict bool
		want   int
	}{"default": {false, defaultRounds}, "strict": {true, strictRounds}} {
		t.Run(name, func(t *testing.T) {
			policy := domain.Policy{StrictMinor: tc.strict, MaxReviewRounds: len(rows) + 1}
			run := domain.Run{Phase: domain.PhaseReviewing}
			for _, row := range rows {
				run.Phase = domain.PhaseReviewing
				run = rework(t, run, policy, finding("F1", row.Severity))
			}
			if run.ReviewRound != tc.want {
				t.Fatalf("got %d rounds over %d rows, want %d", run.ReviewRound, len(rows), tc.want)
			}
		})
	}
}
