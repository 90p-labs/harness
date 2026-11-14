package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	mapFixture      = "../../internal/plan/domain/testdata/map.md"
	mapFrontierOne  = "frontier: T3 job-queue (prototype)\n"
	mapFrontierTwo  = "frontier: T7 bucket-access (work)\n"
	mapBlockedGood  = "- Blocked by: T1 and T2"
	mapBlockedBad   = "- Blocked by: T9"
	mapViewHint     = "harness plan check --file"
	mapRuleBlocked  = "map-blocked"
	mapProblemsText = "plan has problems"
	mapRuleField    = "map-field"
	mapClear        = "map: clear"
	mapUnspecified  = "## Not yet specified\n"
	mapNotes        = "## Notes\n"
)

func writeMap(t *testing.T, text string) string {
	t.Helper()
	file := filepath.Join(initRepo(t), "map.md")
	if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestPlanMap(t *testing.T) {
	clearHarnessEnv(t)
	clearOTLPEnv(t)
	raw, err := os.ReadFile(mapFixture)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("a clean map prints its frontier and no plan lines", func(t *testing.T) {
		file := writeMap(t, string(raw))

		code, stdout, stderr := execute(t, commandPlan, commandCheck, flagFile, file)

		if code != exitOK || stderr != "" || stdout != mapFrontierOne+mapFrontierTwo {
			t.Fatalf("check: code = %d, stdout = %q, stderr = %q, want the two frontier lines", code, stdout, stderr)
		}
		if strings.Contains(stdout, "advisor:") || strings.Contains(stdout, "wave") {
			t.Errorf("stdout = %q, want no advisor and no wave line", stdout)
		}
	})

	t.Run("a broken map lists the problem and fails", func(t *testing.T) {
		file := writeMap(t, strings.Replace(string(raw), mapBlockedGood, mapBlockedBad, 1))

		code, stdout, stderr := execute(t, commandPlan, commandCheck, flagFile, file)

		if code != exitRuntime || !strings.Contains(stdout, mapRuleBlocked) || !strings.Contains(stderr, mapProblemsText) {
			t.Fatalf("check: code = %d, stdout = %q, stderr = %q, want %s and %q", code, stdout, stderr, mapRuleBlocked, mapProblemsText)
		}
		if strings.Contains(stdout, "frontier:") {
			t.Errorf("stdout = %q, want no frontier line", stdout)
		}
	})

	hidden := []struct{ name, old, add string }{
		{"a fence under not yet specified", mapUnspecified, mapUnspecified + "\n```md\n- hidden item\n```\n"},
		{"a near-miss ticket heading in the notes", mapNotes, mapNotes + "\n### T8 Hidden-one\n"},
		{"a level four ticket heading in the notes", mapNotes, mapNotes + "\n#### T8 x\n"},
		{"a hyphen ticket heading in the notes", mapNotes, mapNotes + "\n### T8-hidden\n"},
	}
	for _, tc := range hidden {
		t.Run(tc.name+" fails the check", func(t *testing.T) {
			file := writeMap(t, strings.Replace(string(raw), tc.old, tc.add, 1))

			code, stdout, _ := execute(t, commandPlan, commandCheck, flagFile, file)

			if code != exitRuntime || !strings.Contains(stdout, mapRuleField) {
				t.Fatalf("check: code = %d, stdout = %q, want %d and %s", code, stdout, exitRuntime, mapRuleField)
			}
			if strings.Contains(stdout, "frontier:") || strings.Contains(stdout, mapClear) {
				t.Errorf("stdout = %q, want no frontier line and no %q", stdout, mapClear)
			}
		})
	}

	t.Run("run plan refuses a map with plan-kind", func(t *testing.T) {
		dir, _ := tracedRepo(t)
		_, started, _ := execute(t, commandRun, "start", "Chart the map")
		writeRunPlan(t, dir, startedRunID(t, started), string(raw))

		code, stdout, stderr := execute(t, commandRun, commandPlan)

		if code != exitRuntime || !strings.Contains(stdout+stderr, "plan-kind") || !strings.Contains(stdout+stderr, "not a map") {
			t.Fatalf("run plan: code = %d, stdout = %q, stderr = %q, want the plan-kind refusal", code, stdout, stderr)
		}
	})

	t.Run("the view refuses a map and names the check", func(t *testing.T) {
		file := writeMap(t, string(raw))
		out := filepath.Join(t.TempDir(), "page.html")

		code, stdout, stderr := execute(t, commandPlan, "view", flagFile, file, flagOut, out)

		if code != exitRuntime || stdout != "" || !strings.Contains(stderr, "error: ") || !strings.Contains(stderr, mapViewHint) {
			t.Fatalf("view: code = %d, stdout = %q, stderr = %q, want an error line with %q", code, stdout, stderr, mapViewHint)
		}
		if _, err := os.Stat(out); err == nil {
			t.Error("the view wrote a page for a map")
		}
	})
}
