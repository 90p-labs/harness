package domain_test

import (
	"reflect"
	"testing"

	"github.com/jorge07/harness/internal/plan/domain"
)

func traceOf(t *testing.T, text string) domain.Links {
	t.Helper()
	doc, problems := domain.Parse(text)
	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	return domain.Trace(doc)
}

func TestTraceLinks(t *testing.T) {
	links := traceOf(t, readValid(t))

	wantCriteria := map[int]domain.CriterionLinks{
		1: {
			Tasks: []int{1}, Specs: []string{"Ping domain"},
			Proof:   "go test ./internal/ping/domain -run TestPong -count=1",
			Commits: []int{1, 3}, PR: 1,
		},
		2: {
			Tasks: []int{2}, Specs: []string{"Ping query"},
			Proof:   "go test ./internal/ping/application/query/pingquery -run TestPingQuery -count=1",
			Commits: []int{2, 3}, PR: 1,
			Signal: &domain.SignalRow{
				Criterion: 2, Signal: "ping calls", Source: "prometheus", Query: "ping:calls:rate5m",
				Breach: "> 100", Window: "24h", Panel: "ping calls", Line: 43,
			},
		},
		3: {
			Tasks: []int{2}, Specs: []string{"Ping query"},
			Proof:   "make check",
			Commits: []int{2, 3}, PR: 1,
		},
	}
	if !reflect.DeepEqual(links.Criteria, wantCriteria) {
		t.Errorf("criteria links = %+v\nwant %+v", links.Criteria, wantCriteria)
	}

	wantTasks := map[int]domain.TaskLinks{
		1: {Files: []string{"internal/ping/domain/pong.go", "internal/ping/domain/pong_test.go"}, Test: "TestPong"},
		2: {Files: []string{"internal/ping/application/query/pingquery/handler.go", "internal/ping/application/query/pingquery/handler_test.go"}, Test: "TestPingQuery"},
	}
	if !reflect.DeepEqual(links.Tasks, wantTasks) {
		t.Errorf("task links = %+v\nwant %+v", links.Tasks, wantTasks)
	}
	if len(links.Unlinked) != 0 {
		t.Errorf("unlinked = %v", links.Unlinked)
	}
}

func TestTraceUnlinked(t *testing.T) {
	valid := readValid(t)
	tests := []struct {
		name  string
		edits []edit
		want  []string
	}{
		{"criterion no spec names", []edit{{"Criteria: 2 and 3", "Criteria: 2"}}, []string{"criterion 3"}},
		{"task owns no tree file", []edit{
			{"`internal/ping/domain/pong.go`, `internal/ping/domain/pong_test.go`", "`internal/ping/elsewhere/`"},
		}, []string{"task 1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			links := traceOf(t, mutate(t, valid, tc.edits...))
			if !reflect.DeepEqual(links.Unlinked, tc.want) {
				t.Errorf("unlinked = %v, want %v", links.Unlinked, tc.want)
			}
		})
	}
}

func TestTraceRules(t *testing.T) {
	valid := readValid(t)
	tests := []struct {
		name  string
		edits []edit
		rule  domain.RuleID
		line  int
	}{
		{"spec without a Criteria line", []edit{{"Criteria: 2 and 3\n", ""}}, domain.RuleSpecCriteria, 70},
		{"spec names a missing criterion", []edit{{"Criteria: 1\n", "Criteria: 1 and 9\n"}}, domain.RuleSpecCriteria, 52},
		{"criterion named by no spec", []edit{{"Criteria: 2 and 3", "Criteria: 2"}}, domain.RuleTraceUnlinked, 19},
		{"task owns no tree file", []edit{
			{"`internal/ping/domain/pong.go`, `internal/ping/domain/pong_test.go`", "`internal/ping/elsewhere/`"},
		}, domain.RuleTraceUnlinked, 23},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := byRule(checkText(mutate(t, valid, tc.edits...), domain.CheckInput{}), tc.rule)
			if len(got) != 1 || got[0].Line != tc.line {
				t.Errorf("%s problems = %v, want one at line %d", tc.rule, got, tc.line)
			}
		})
	}
}
