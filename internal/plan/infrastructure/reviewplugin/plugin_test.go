package reviewplugin_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/plan/domain"
	"github.com/jorge07/harness/internal/plan/infrastructure/reviewplugin"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	runID      = "run-1"
	baseBranch = "develop"
	mainBranch = "main"
	planText   = "# Plan: a\n" +
		"\n## Acceptance criteria\n\n| # | Criterion | Proof |\n|---|---|---|\n| 1 | It works. | `make check` |\n" +
		"\n## Tasks\n\n### 1. Work\n\n- Side: backend\n- Owned files: `pkg/a.go`, `pkg/a_test.go`\n" +
		"- Test first: `a_test.go`: `TestA` fails.\n- DoD: criterion 1.\n- Depends on: none\n" +
		"\n## Tech specs\n\n### Final code tree\n\n```tree\nrepo/\n└── pkg/\n    └── a.go    new\n```\n" +
		"\n### Final test tree\n\n```tree\nrepo/\n└── pkg/\n    └── a_test.go    new\n```\n" +
		"\n### Commits\n\n1. `feat: add a` (task 1)\n" +
		"\n### PR sequence\n\n1. `feat/a` (this run)%s: the work.\n"
)

type branches struct {
	branch domain.Branch
	base   string
	tests  []string
}

func (b *branches) Read(_ context.Context, _, base string, tests []string) (domain.Branch, error) {
	b.base, b.tests = base, tests
	return b.branch, nil
}

type proofs map[int]bool

func (p proofs) Passed(context.Context) (map[int]bool, error) { return p, nil }

func cleanBranch() domain.Branch {
	return domain.Branch{
		Files: []domain.FileChange{
			{Path: "pkg/a.go", Status: domain.StatusAdded},
			{Path: "pkg/a_test.go", Status: domain.StatusAdded},
		},
		Commits: []domain.BranchCommit{{SHA: "a1", Subject: "feat: add a", Files: []string{"pkg/a.go", "pkg/a_test.go"}}},
		Tests:   map[string]string{"pkg/a_test.go": "func TestA(t *testing.T) {}\n"},
	}
}

func envelope(t *testing.T, worktree, event string) shareddomain.Envelope {
	t.Helper()
	raw, err := json.Marshal(shareddomain.TransitionPayload{Event: event})
	if err != nil {
		t.Fatal(err)
	}
	return shareddomain.Envelope{
		Version: shareddomain.EnvelopeVersion, Event: shareddomain.EventRunTransitionBefore,
		Cwd: worktree, Run: shareddomain.RunRef{ID: runID}, Payload: raw,
	}
}

func worktreeWith(t *testing.T, planBase string) string {
	t.Helper()
	dir := t.TempDir()
	file := filepath.Join(dir, filepath.FromSlash(shareddomain.RunPlanFile(runID)))
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(strings.Replace(planText, "%s", planBase, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func newPlugin(dir string, b *branches, p proofs) shareddomain.Plugin {
	defaultBranch := func(context.Context) (string, error) { return mainBranch, nil }
	return reviewplugin.New(dir, b, p, defaultBranch)
}

func TestReviewPlugin(t *testing.T) {
	ctx := context.Background()

	t.Run("subscribes to the transition veto", func(t *testing.T) {
		p := newPlugin(t.TempDir(), &branches{}, proofs{})
		if got := p.Subscribes(); !slices.Equal(got, []shareddomain.EventName{shareddomain.EventRunTransitionBefore}) {
			t.Fatalf("Subscribes = %v", got)
		}
	})

	t.Run("denies with the problems and reads from the plan base", func(t *testing.T) {
		dir := worktreeWith(t, ", based on `"+baseBranch+"`")
		b := &branches{branch: cleanBranch()}
		b.branch.Commits[0].Subject = "feat: other"

		verdict, err := newPlugin(dir, b, proofs{1: false}).Handle(ctx, envelope(t, dir, shareddomain.TransitionSubmitForReview))

		if err != nil {
			t.Fatal(err)
		}
		if verdict.Kind != shareddomain.VerdictDeny {
			t.Fatalf("verdict = %+v", verdict)
		}
		for _, rule := range []domain.RuleID{domain.RuleCommitMissing, domain.RuleProofMissing} {
			if !strings.Contains(verdict.Reason, string(rule)) {
				t.Errorf("reason %q does not name %s", verdict.Reason, rule)
			}
		}
		if lines := strings.Split(verdict.Reason, "\n"); len(lines) != 3 {
			t.Errorf("reason has %d lines, want 2 problems and the next step: %q", len(lines), verdict.Reason)
		}
		if b.base != baseBranch || !slices.Equal(b.tests, []string{"pkg/a_test.go"}) {
			t.Errorf("read base %q tests %v", b.base, b.tests)
		}
	})

	t.Run("allows a clean branch from the default base", func(t *testing.T) {
		dir := worktreeWith(t, "")
		b := &branches{branch: cleanBranch()}

		verdict, err := newPlugin(dir, b, proofs{1: true}).Handle(ctx, envelope(t, dir, shareddomain.TransitionSubmitForReview))

		if err != nil || verdict.Kind != shareddomain.VerdictAllow {
			t.Fatalf("verdict = %+v, err = %v", verdict, err)
		}
		if b.base != mainBranch {
			t.Errorf("read base %q, want %q", b.base, mainBranch)
		}
	})

	t.Run("denies a plan with parse problems", func(t *testing.T) {
		dir := worktreeWith(t, "")
		file := filepath.Join(dir, filepath.FromSlash(shareddomain.RunPlanFile(runID)))
		broken := strings.Replace(strings.Replace(planText, "%s", "", 1), "| 1 | It works.", "| one | It works.", 1)
		if err := os.WriteFile(file, []byte(broken), 0o644); err != nil {
			t.Fatal(err)
		}

		verdict, err := newPlugin(dir, &branches{branch: cleanBranch()}, proofs{}).Handle(ctx, envelope(t, dir, shareddomain.TransitionSubmitForReview))

		if err != nil || verdict.Kind != shareddomain.VerdictDeny || !strings.Contains(verdict.Reason, `"one"`) {
			t.Fatalf("verdict = %+v, err = %v, want a deny that names the bad row", verdict, err)
		}
	})

	t.Run("ignores the other events", func(t *testing.T) {
		dir := t.TempDir()
		b := &branches{}

		verdict, err := newPlugin(dir, b, proofs{}).Handle(ctx, envelope(t, dir, "request-rework"))

		if err != nil || verdict.Kind != shareddomain.VerdictAllow || b.base != "" {
			t.Fatalf("verdict = %+v, err = %v, base = %q", verdict, err, b.base)
		}
	})
}
