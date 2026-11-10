package domain_test

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

const programFixture = "testdata/program.md"

func readProgram(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(programFixture)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func checkProgramText(text string) []domain.Problem {
	p, problems := domain.ParseProgram(text)
	problems = append(problems, domain.CheckProgram(p)...)
	domain.SortProblems(problems)
	return problems
}

func TestParseProgram(t *testing.T) {
	p, problems := domain.ParseProgram(readProgram(t))
	if len(problems) != 0 {
		t.Fatalf("fixture has parse problems: %v", problems)
	}
	if p.Title != "Harness improvement plan" {
		t.Errorf("title = %q", p.Title)
	}
	if len(p.Goal) != 2 || !strings.HasPrefix(p.Goal[0], "Eight harness runs") {
		t.Errorf("goal = %q", p.Goal)
	}
	wantRuns := []struct {
		id, slug, branch, doneWhen string
		wave, scope                int
		depends, owned             []string
	}{
		{"R1", "guard-precision", "feat/guard-precision", "a replay of the denied commands matches the expected result after the fix.", 1, 3, nil,
			[]string{"internal/guard/domain/shell.go", "internal/guard/domain/rules_files.go", "internal/guard/domain/rules_pr.go"}},
		{"R2", "prose-gate", "feat/prose-gate", "the word-count reasons of the rework log fail the gate in the working phase.", 1, 2, nil,
			[]string{"internal/plan/domain/sentence.go", "skills/skills_test.go", ".harness.yaml"}},
		{"R3a", "review-brief", "docs/review-brief", "a review of a sample diff returns all findings in one report with a severity on each.", 1, 2, nil,
			[]string{"skills/harness/SKILL.md", "agents/reviewer.md"}},
		{"R4", "spec-quality", "feat/spec-quality", "a replay of two old plans gives the known gaps as open questions at plan time.", 2, 3, []string{"R2"},
			[]string{"agents/explorer.md", "agents/advisor.md", "internal/plan/domain/check.go"}},
		{"R5", "coordinator", "feat/coordinator", "the lead watches all runs with 1 command, and a merge is 1 command.", 2, 3, []string{"R1"},
			[]string{"internal/run/ui/cli/list.go", "internal/run/domain/approval.go"}},
		{"R6", "telemetry-and-stop", "feat/telemetry-and-stop", "the audit counts come from 1 Jaeger query and 1 command.", 2, 2, []string{"R1", "R3a"},
			[]string{"internal/run/domain/stop.go", "internal/run/application/query/report/"}},
		{"R3b", "review-loop", "feat/review-loop", "a rebase after the review needs no new review, and a run with only minor findings ships after 1 round.", 3, 2, []string{"R3a", "R4"},
			[]string{"internal/run/domain/guard.go", "internal/run/infrastructure/sqlite/migrations/"}},
		{"R7", "lead-cost", "feat/lead-cost", "the median context of a run lead call falls by 30% on 3 runs.", 3, 2, []string{"R1", "R2", "R4"},
			[]string{"skills/lead/SKILL.md", "internal/verify/domain/output.go"}},
	}
	if len(p.Runs) != len(wantRuns) {
		t.Fatalf("runs = %d, want %d", len(p.Runs), len(wantRuns))
	}
	for i, want := range wantRuns {
		t.Run(want.id, func(t *testing.T) {
			r := p.Runs[i]
			if r.ID != want.id || r.Slug != want.slug || r.Wave != want.wave || r.Branch != want.branch || r.DoneWhen != want.doneWhen {
				t.Errorf("run = %+v", r)
			}
			if !slices.Equal(r.DependsOn, want.depends) || !slices.Equal(r.Owned, want.owned) {
				t.Errorf("depends = %v, owned = %v", r.DependsOn, r.Owned)
			}
			if len(r.Scope) != want.scope {
				t.Errorf("scope = %d steps, want %d", len(r.Scope), want.scope)
			}
			if r.Line == 0 || len(r.FieldLines) != len(domain.ProgramFields) {
				t.Errorf("line = %d, field lines = %v", r.Line, r.FieldLines)
			}
		})
	}
	if !slices.Equal(p.MeasurementHeader, []string{"Metric", "Baseline", "Target"}) {
		t.Errorf("measurement header = %v", p.MeasurementHeader)
	}
	if len(p.MeasurementRows) != 4 || !slices.Equal(p.MeasurementRows[1], []string{"Round-limit blocks", "9", "0"}) {
		t.Errorf("measurement rows = %v", p.MeasurementRows)
	}
	if len(p.Decisions) != 3 || !strings.HasPrefix(p.Decisions[2], "The coordinator runs") {
		t.Errorf("decisions = %q", p.Decisions)
	}
}

func TestCheckProgram(t *testing.T) {
	valid := readProgram(t)
	if got := checkProgramText(valid); len(got) != 0 {
		t.Fatalf("fixture has problems: %v", got)
	}

	longSentence := strings.TrimSpace(strings.Repeat("word ", 26)) + "."
	const (
		r1Fields = "- Wave: 1\n- Depends on: none\n- Branch: feat/guard-precision"
		r5Owned  = "- Owned files: `internal/run/ui/cli/list.go`, `internal/run/domain/approval.go`"
		r7Scope  = "1. `harness verify` prints only the failed gates and 1 summary line by default.\n2. The skill tells the lead to read diffs with `--stat` first.\n"
	)
	tests := []struct {
		name  string
		text  string
		edits []edit
		rule  domain.RuleID
		line  func(text string) int
	}{
		{"no goal", valid, []edit{{"## Goal", "## Aim"}}, domain.RuleHeadingMissing, func(string) int { return 1 }},
		{"no runs heading", valid, []edit{{"## Runs", "## Plan"}}, domain.RuleHeadingMissing, func(string) int { return 1 }},
		{"runs without a run", "# T\n\n## Goal\n\nText.\n\n## Runs\n", nil, domain.RuleHeadingMissing, func(string) int { return 7 }},
		{"bad run heading", valid, []edit{{"### R7 lead-cost", "### Run seven"}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "### Run seven") }},
		{"duplicate id", valid, []edit{{"### R7 lead-cost", "### R1 lead-cost"}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "### R1 lead-cost") }},
		{"missing branch", valid, []edit{{"\n- Branch: feat/guard-precision", ""}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "### R1 guard-precision") }},
		{"wave not a number", valid, []edit{{r1Fields, strings.Replace(r1Fields, "Wave: 1", "Wave: soon", 1)}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "Wave: soon") }},
		{"wave zero", valid, []edit{{r1Fields, strings.Replace(r1Fields, "Wave: 1", "Wave: 0", 1)}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "Wave: 0") }},
		{"bad branch", valid, []edit{{"feat/guard-precision", "guard-precision"}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "Branch: guard-precision") }},
		{"no owned path", valid, []edit{{r5Owned, "- Owned files: none"}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "Owned files: none") }},
		{"no scope step", valid, []edit{{r7Scope, ""}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "### R7 lead-cost") }},
		{"blank dependency", valid, []edit{{r1Fields, strings.Replace(r1Fields, "Depends on: none", "Depends on:", 1)}}, domain.RuleRunField, func(s string) int { return lineOf(t, s, "Depends on:") }},
		{"unknown dependency", valid, []edit{{"Depends on: R2\n", "Depends on: R9\n"}}, domain.RuleRunDepends, func(s string) int { return lineOf(t, s, "Depends on: R9") }},
		{"same wave dependency", valid, []edit{{"Depends on: R1 and R3a", "Depends on: R1 and R5"}}, domain.RuleRunDepends, func(s string) int { return lineOf(t, s, "Depends on: R1 and R5") }},
		{"later wave dependency", valid, []edit{{"Depends on: R2\n", "Depends on: R7\n"}}, domain.RuleRunDepends, func(s string) int { return lineOf(t, s, "Depends on: R7") }},
		{"overlap on a file", valid, []edit{{"`internal/run/domain/approval.go`", "`internal/run/domain/stop.go`"}}, domain.RuleRunOverlap, func(s string) int { return lineOf(t, s, "query/report/`") }},
		{"overlap on a folder", valid, []edit{{"`internal/run/domain/approval.go`", "`internal/run/application/query/report/list.go`"}}, domain.RuleRunOverlap, func(s string) int { return lineOf(t, s, "query/report/`") }},
		{"long sentence", valid, []edit{{"2. Each deny names the resolved file and the line number.", "2. " + longSentence}}, domain.RuleSentenceLength, func(s string) int { return lineOf(t, s, "2. word word") }},
		{"em dash", valid, []edit{{"Each deny names", "Each deny \u2014 names"}}, domain.RuleEmDash, func(s string) int { return lineOf(t, s, "\u2014") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.text
			if len(tc.edits) > 0 {
				text = mutate(t, text, tc.edits...)
			}
			got := checkProgramText(text)
			want := fmt.Sprintf("plan.md:%d: %s:", tc.line(text), tc.rule)
			if len(got) != 1 || !strings.HasPrefix(got[0].String(), want) {
				t.Fatalf("problems = %v, want one %q", got, want)
			}
		})
	}
}

func TestCheckProgramOverlapNamesRunsAndPath(t *testing.T) {
	text := mutate(t, readProgram(t), edit{"`internal/run/domain/approval.go`", "`internal/run/domain/stop.go`"})
	got := byRule(checkProgramText(text), domain.RuleRunOverlap)
	if len(got) != 1 {
		t.Fatalf("overlap problems = %v", got)
	}
	for _, part := range []string{"R5", "R6", "internal/run/domain/stop.go"} {
		if !strings.Contains(got[0].Message, part) {
			t.Errorf("message %q lacks %q", got[0].Message, part)
		}
	}
}

func TestIsProgram(t *testing.T) {
	valid, err := os.ReadFile("testdata/valid.md")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"program fixture", readProgram(t), true},
		{"single-run plan", string(valid), false},
		{"runs line inside a fence", "# T\n\n```md\n## Runs\n```\n", false},
		{"runs line after a fence", "# T\n\n```md\ntext\n```\n\n## Runs\n", true},
		{"runs heading with text", "# T\n\n## Runs and more\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.IsProgram(tc.text); got != tc.want {
				t.Errorf("IsProgram = %v, want %v", got, tc.want)
			}
		})
	}
}
