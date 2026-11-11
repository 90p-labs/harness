package domain_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/shared/domain"
)

const findingsHeader = "| ID | Severity | File:line | Finding | Verdict | Status | Action |\n|---|---|---|---|---|---|---|\n"

func TestParseReviewFindings(t *testing.T) {
	t.Run("reads the table among other text and tables", func(t *testing.T) {
		text := "# Findings\n\n| Name | Note |\n|---|---|\n| a | b |\n\n" + findingsHeader +
			"| F1 | Blocker | a.go:1 | bad | real | open | fix |\n" +
			"| F2 | minor | b.go:2 | nit | Not Real | closed | none |\n\nafter\n"
		got, err := domain.ParseReviewFindings(text)
		if err != nil {
			t.Fatal(err)
		}
		want := []domain.ReviewFinding{
			{ID: "F1", Severity: domain.SeverityBlocker, Verdict: domain.FindingReal, Status: domain.FindingOpen},
			{ID: "F2", Severity: domain.SeverityMinor, Verdict: domain.FindingNotReal, Status: domain.FindingClosed},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("reads every findings table", func(t *testing.T) {
		text := findingsHeader + "| F1 | minor | a | b | real | closed | c |\n\ntext\n\n" + findingsHeader +
			"| F2 | blocker | a | b | real | open | c |\n"
		got, err := domain.ParseReviewFindings(text)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[1].ID != "F2" || got[1].Severity != domain.SeverityBlocker {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("skips a table with none of the findings columns", func(t *testing.T) {
		text := "| # | Proof | Exit |\n|---|---|---|\n| 1 | x | 0 |\n\n" + findingsHeader +
			"| F1 | blocker | a | b | real | open | c |\n"
		got, err := domain.ParseReviewFindings(text)
		if err != nil || len(got) != 1 {
			t.Fatalf("got %+v, %v", got, err)
		}
	})

	t.Run("a duplicate ID across tables is an error", func(t *testing.T) {
		text := findingsHeader + "| F1 | minor | a | b | real | closed | c |\n\n" + findingsHeader +
			"| F1 | blocker | a | b | real | open | c |\n"
		if _, err := domain.ParseReviewFindings(text); err == nil || !strings.Contains(err.Error(), "duplicate") {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("reads the columns in any order", func(t *testing.T) {
		text := "| status | VERDICT | severity | id |\n|---|---|---|---|\n| open | real | major | X1 |\n"
		got, err := domain.ParseReviewFindings(text)
		if err != nil {
			t.Fatal(err)
		}
		want := []domain.ReviewFinding{{ID: "X1", Severity: domain.SeverityMajor, Verdict: domain.FindingReal, Status: domain.FindingOpen}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	errCases := map[string]struct{ text, want string }{
		"unknown severity": {findingsHeader + "| F1 | critical | a | b | real | open | c |\n", "line 3"},
		"unknown verdict":  {findingsHeader + "| F1 | major | a | b | maybe | open | c |\n", "line 3"},
		"unknown status":   {findingsHeader + "| F1 | major | a | b | real | wip | c |\n", "line 3"},
		"duplicate id": {findingsHeader +
			"| F1 | major | a | b | real | open | c |\n| F1 | minor | a | b | real | open | c |\n", "line 4"},
		"short row":      {findingsHeader + "| F1 | major |\n", "line 3"},
		"missing header": {"| ID | Severity |\n|---|---|\n| F1 | major |\n", "line 1: missing the columns verdict, status"},
		"valid then misspelled table": {findingsHeader + "| F1 | minor | a | b | real | closed | c |\n\n" +
			"| ID | Severity | Verdict | Staus |\n|---|---|---|---|\n| F2 | blocker | real | open |\n", "line 5: missing the columns status"},
		"misspelled then valid table": {"| ID | Severity | Verdict | Staus |\n|---|---|---|---|\n| F2 | blocker | real | open |\n\n" +
			findingsHeader + "| F1 | minor | a | b | real | closed | c |\n", "line 1: missing the columns status"},
		"unrelated table only": {"| Check | Result | Evidence |\n|---|---|---|\n| a | b | c |\n", "no findings table"},
		"no table":             {"nothing here\n", "no findings table"},
	}
	for name, tc := range errCases {
		t.Run(name, func(t *testing.T) {
			_, err := domain.ParseReviewFindings(tc.text)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want an error with %q", err, tc.want)
			}
		})
	}
}

func TestNamesFinding(t *testing.T) {
	for name, tc := range map[string]struct {
		reason string
		want   bool
	}{
		"comma":     {"fix F1, F2", true},
		"semicolon": {"F2; F1; x", true},
		"space":     {"fix F1 now", true},
		"end":       {"fix F1", true},
		"start":     {"F1", true},
		"longer ID": {"fix F10", false},
		"suffix":    {"fix F1x", false},
		"prefix":    {"fix XF1", false},
		"empty":     {"", false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := domain.NamesFinding(tc.reason, "F1"); got != tc.want {
				t.Fatalf("NamesFinding(%q) = %v", tc.reason, got)
			}
		})
	}
}

func TestParseReviewFindingsModel(t *testing.T) {
	text := "| ID | Severity | Verdict | Status | model |\n|---|---|---|---|---|\n| F1 | major | real | open | opus |\n| F2 | minor | real | open |\n"
	got, err := domain.ParseReviewFindings(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Model != "opus" || got[1].Model != "" {
		t.Fatalf("got %+v, want the Model cell of F1 and none for F2", got)
	}
	plain, err := domain.ParseReviewFindings("| ID | Severity | Verdict | Status |\n|---|---|---|---|\n| F1 | major | real | open |\n")
	if err != nil || plain[0].Model != "" {
		t.Fatalf("without the column: got %+v, %v, want no model", plain, err)
	}
}
