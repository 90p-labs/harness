package domain_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/gate/domain"
)

func TestJudgeCommits(t *testing.T) {
	cases := []struct {
		name    string
		commits []domain.CommitResult
		wantSHA string
	}{
		{
			name: "failing test then passing feat",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeTest, Passed: false},
				{SHA: "b2", Type: domain.CommitTypeFeat, Passed: true},
			},
		},
		{
			name: "failing test then passing fix and passing chore",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeTest, Passed: false},
				{SHA: "b2", Type: domain.CommitTypeFix, Passed: true},
				{SHA: "c3", Type: "chore", Passed: true},
			},
		},
		{
			name: "passing test with a feat in the range",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeTest, Passed: true},
				{SHA: "b2", Type: domain.CommitTypeFeat, Passed: true},
			},
			wantSHA: "a1",
		},
		{
			name: "failing test with no later commit",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeFeat, Passed: true},
				{SHA: "b2", Type: domain.CommitTypeTest, Passed: false},
			},
			wantSHA: "b2",
		},
		{
			name: "failing test with only failing later commit",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeTest, Passed: false},
				{SHA: "b2", Type: domain.CommitTypeTest, Passed: false},
				{SHA: "c3", Type: domain.CommitTypeFeat, Passed: false},
			},
			wantSHA: "a1",
		},
		{
			name: "failing fix",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeFix, Passed: false},
			},
			wantSHA: "a1",
		},
		{
			name: "failing other commit",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeFeat, Passed: true},
				{SHA: "b2", Type: "docs", Passed: false},
			},
			wantSHA: "b2",
		},
		{
			name: "no feat or fix and passing test",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeTest, Passed: true},
				{SHA: "b2", Type: "chore", Passed: true},
			},
		},
		{
			name: "no feat or fix and failing test",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: "chore", Passed: true},
				{SHA: "b2", Type: domain.CommitTypeTest, Passed: false},
			},
			wantSHA: "b2",
		},
		{name: "empty range"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.JudgeCommits(tc.commits)
			if tc.wantSHA == "" {
				if err != nil {
					t.Fatalf("JudgeCommits error = %v", err)
				}
				return
			}
			if !errors.Is(err, domain.ErrCommitRule) {
				t.Fatalf("error = %v, want ErrCommitRule", err)
			}
			if !strings.Contains(err.Error(), tc.wantSHA) {
				t.Fatalf("error %q does not name %s", err, tc.wantSHA)
			}
		})
	}
}

func TestJudgeCommitsNamesTheFailedGates(t *testing.T) {
	failed := []domain.GateFailure{
		{Gate: "test", Status: domain.StatusFail, Duration: 12 * time.Millisecond, Tail: "FAIL: TestWidget"},
		{Gate: "lint", Status: domain.StatusVetoed},
	}
	cases := []struct {
		name    string
		commits []domain.CommitResult
		want    string
		absent  string
	}{
		{
			name:    "must pass names the failed gates in order",
			commits: []domain.CommitResult{{SHA: "a1", Type: domain.CommitTypeFeat, Failed: failed}},
			want:    "commit a1 (feat) must pass: failed test, lint",
		},
		{
			name:    "must pass with no failed gates keeps the message",
			commits: []domain.CommitResult{{SHA: "a1", Type: domain.CommitTypeFeat}},
			want:    "commit a1 (feat) must pass",
			absent:  "failed",
		},
		{
			name: "other messages do not name gates",
			commits: []domain.CommitResult{
				{SHA: "a1", Type: domain.CommitTypeTest, Passed: true},
				{SHA: "b2", Type: domain.CommitTypeFeat, Passed: true},
			},
			want:   "commit a1 (test) must fail before the code that fixes it",
			absent: "failed ",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := domain.JudgeCommits(tc.commits)
			if !errors.Is(err, domain.ErrCommitRule) {
				t.Fatalf("error = %v, want ErrCommitRule", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
			if tc.absent != "" && strings.Contains(strings.TrimPrefix(err.Error(), domain.ErrCommitRule.Error()), tc.absent) {
				t.Fatalf("error %q contains %q", err, tc.absent)
			}
		})
	}
}

func TestCommitType(t *testing.T) {
	cases := map[string]string{
		"test: reproduce the crash":      domain.CommitTypeTest,
		"feat(gate): add the domain":     domain.CommitTypeFeat,
		"fix!: drop the old field":       domain.CommitTypeFix,
		"chore(deps)!: bump":             "chore",
		"Merge branch 'main' into topic": "",
		"":                               "",
	}
	for subject, want := range cases {
		if got := domain.CommitType(subject); got != want {
			t.Errorf("CommitType(%q) = %q, want %q", subject, got, want)
		}
	}
}
