package baseline_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jorge07/harness/internal/lint/application/command/baseline"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const baselinePath = ".harness-baseline/lint.json"

var errLint = errors.New("lint failed")

func finding(rule, file string) shareddomain.Finding {
	return shareddomain.Finding{Rule: rule, Severity: shareddomain.SeverityError, File: file, Line: 7, Message: rule + " broken"}
}

func lintReturning(findings ...shareddomain.Finding) baseline.Lint {
	return func(context.Context, string) (shareddomain.Report, error) {
		return shareddomain.Report{Version: shareddomain.ReportVersion, Findings: findings}, nil
	}
}

func readReport(t *testing.T, dir string) (shareddomain.Report, string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, baselinePath))
	if err != nil {
		t.Fatal(err)
	}
	var report shareddomain.Report
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatal(err)
	}
	return report, string(raw)
}

func write(t *testing.T, dir string, lint baseline.Lint) {
	t.Helper()
	if err := baseline.NewHandler(lint).Handle(context.Background(), baseline.WriteBaseline{Dir: dir, Path: baselinePath}); err != nil {
		t.Fatal(err)
	}
}

func TestBaselineRatchet(t *testing.T) {
	dir := t.TempDir()
	a, b, c := finding("ddd/a", "a.ts"), finding("ddd/b", "b.ts"), finding("ddd/c", "c.ts")

	write(t, dir, lintReturning(a, b))
	first, raw := readReport(t, dir)
	if first.Version != 1 || len(first.Findings) != 2 {
		t.Fatalf("first baseline = %+v", first)
	}
	if want := "{\n  \"version\": 1,\n  \"findings\": [\n"; raw[:len(want)] != want || raw[len(raw)-1] != '\n' {
		t.Fatalf("baseline is not indented with a trailing newline: %q", raw)
	}

	write(t, dir, lintReturning(a, c))
	second, _ := readReport(t, dir)
	if len(second.Findings) != 1 || second.Findings[0] != a {
		t.Fatalf("second baseline = %+v, want only the finding that is still found", second)
	}

	write(t, dir, lintReturning())
	third, _ := readReport(t, dir)
	if third.Findings == nil || len(third.Findings) != 0 {
		t.Fatalf("third baseline = %+v, want an empty list", third)
	}
}

func TestBaselineErrors(t *testing.T) {
	t.Run("a lint error writes nothing", func(t *testing.T) {
		dir := t.TempDir()
		lint := func(context.Context, string) (shareddomain.Report, error) { return shareddomain.Report{}, errLint }

		err := baseline.NewHandler(lint).Handle(context.Background(), baseline.WriteBaseline{Dir: dir, Path: baselinePath})

		if !errors.Is(err, errLint) {
			t.Fatalf("err = %v", err)
		}
		if _, statErr := os.Stat(filepath.Join(dir, baselinePath)); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("baseline must not exist, stat err = %v", statErr)
		}
	})

	t.Run("a broken baseline file is an error", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, filepath.Dir(baselinePath)), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, baselinePath), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}

		err := baseline.NewHandler(lintReturning()).Handle(context.Background(), baseline.WriteBaseline{Dir: dir, Path: baselinePath})

		if err == nil {
			t.Fatal("want an error for a broken baseline")
		}
	})
}
