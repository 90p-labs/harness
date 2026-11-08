package domain_test

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/plan/domain"
)

const (
	pongFile      = "internal/ping/domain/pong.go"
	pongTestFile  = "internal/ping/domain/pong_test.go"
	legacyFile    = "internal/ping/domain/legacy.go"
	handlerFile   = "internal/ping/application/query/pingquery/handler.go"
	handlerTest   = "internal/ping/application/query/pingquery/handler_test.go"
	moduleFile    = "internal/ping/module.go"
	strayFile     = "internal/ping/stray.go"
	runPlanFile   = ".harness/runs/run-1/plan.md"
	evidenceFile  = ".kodify/runs/plan.md"
	pongSubject   = "feat: add the pong domain"
	querySubject  = "feat: add the ping query"
	missedSubject = "test: cover the ping query"
)

func readFixture(t *testing.T, name string) domain.Document {
	t.Helper()
	raw, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	doc, problems := domain.Parse(string(raw))
	if len(problems) != 0 {
		t.Fatalf("%s parse problems: %v", name, problems)
	}
	return doc
}

func cleanBranch() domain.Branch {
	return domain.Branch{
		Files: []domain.FileChange{
			{Path: pongFile, Status: domain.StatusAdded},
			{Path: pongTestFile, Status: domain.StatusAdded},
			{Path: legacyFile, Status: domain.StatusDeleted},
			{Path: handlerFile, Status: domain.StatusAdded},
			{Path: handlerTest, Status: domain.StatusAdded},
			{Path: moduleFile, Status: domain.StatusModified},
			{Path: runPlanFile, Status: domain.StatusAdded},
			{Path: evidenceFile, Status: domain.StatusAdded},
		},
		Commits: []domain.BranchCommit{
			{SHA: "a1", Subject: pongSubject, Files: []string{pongFile, pongTestFile, legacyFile}},
			{SHA: "b2", Subject: querySubject, Files: []string{handlerFile, handlerTest, moduleFile, runPlanFile}},
		},
		Tests: map[string]string{
			pongTestFile: "package domain_test\n\nfunc TestPong(t *testing.T) {}\n",
			handlerTest:  "package pingquery_test\n\nfunc TestPingQuery(t *testing.T) {}\n",
		},
	}
}

func brokenBranch() domain.Branch {
	b := cleanBranch()
	b.Files[0].Status = domain.StatusModified
	b.Files = append(b.Files, domain.FileChange{Path: strayFile, Status: domain.StatusAdded})
	b.Commits[1].Files = append(b.Commits[1].Files, strayFile)
	b.Tests[handlerTest] = "package pingquery_test\n\nfunc TestOther(t *testing.T) {}\n"
	return b
}

func problemRules(problems []domain.Problem) []domain.RuleID {
	rules := make([]domain.RuleID, len(problems))
	for i, p := range problems {
		rules[i] = p.Rule
	}
	slices.Sort(rules)
	return rules
}

func TestVerifyTrace(t *testing.T) {
	allPass := map[int]bool{1: true, 2: true}

	t.Run("clean fixture has no problem", func(t *testing.T) {
		if got := domain.VerifyTrace(readFixture(t, "verify_clean.md"), cleanBranch(), allPass); len(got) != 0 {
			t.Fatalf("problems = %v", got)
		}
	})

	t.Run("broken fixture has one problem for each rule", func(t *testing.T) {
		proofs := map[int]bool{1: true, 2: false}

		got := domain.VerifyTrace(readFixture(t, "verify_broken.md"), brokenBranch(), proofs)

		want := []domain.RuleID{
			domain.RuleCommitMissing, domain.RuleCommitScope, domain.RuleFileUntracked,
			domain.RuleProofMissing, domain.RuleTestMissing, domain.RuleTreeDiff,
		}
		if rules := problemRules(got); !slices.Equal(rules, want) {
			t.Fatalf("rules = %v, want %v", rules, want)
		}
		subjects := map[domain.RuleID]string{
			domain.RuleTreeDiff:      pongFile,
			domain.RuleFileUntracked: strayFile,
			domain.RuleCommitMissing: missedSubject,
			domain.RuleCommitScope:   strayFile,
			domain.RuleTestMissing:   "TestPingQuery",
			domain.RuleProofMissing:  "criterion 2",
		}
		for _, p := range got {
			if !strings.Contains(p.Message, subjects[p.Rule]) {
				t.Errorf("%s message %q does not name %q", p.Rule, p.Message, subjects[p.Rule])
			}
		}
	})

	t.Run("a test file that the branch does not hold is missing", func(t *testing.T) {
		b := cleanBranch()
		delete(b.Tests, pongTestFile)

		got := domain.VerifyTrace(readFixture(t, "verify_clean.md"), b, allPass)

		if rules := problemRules(got); !slices.Equal(rules, []domain.RuleID{domain.RuleTestMissing}) {
			t.Fatalf("problems = %v", got)
		}
	})

	t.Run("a test file outside the owned files is named", func(t *testing.T) {
		doc := readFixture(t, "verify_clean.md")
		doc.Tasks[0].TestFirst = "`elsewhere_test.go`: `TestPong` fails"

		got := domain.VerifyTrace(doc, cleanBranch(), allPass)

		if len(got) != 1 || got[0].Rule != domain.RuleTestMissing || !strings.Contains(got[0].Message, "elsewhere_test.go") {
			t.Fatalf("problems = %v, want one test-missing problem that names the file", got)
		}
	})

	t.Run("a mark that the diff lacks is a tree diff", func(t *testing.T) {
		b := cleanBranch()
		b.Files = slices.DeleteFunc(b.Files, func(f domain.FileChange) bool { return f.Path == moduleFile })

		got := domain.VerifyTrace(readFixture(t, "verify_clean.md"), b, allPass)

		if rules := problemRules(got); !slices.Equal(rules, []domain.RuleID{domain.RuleTreeDiff}) {
			t.Fatalf("problems = %v", got)
		}
	})
}

func TestTestFiles(t *testing.T) {
	got := domain.TestFiles(readFixture(t, "verify_clean.md"))

	if want := []string{pongTestFile, handlerTest}; !slices.Equal(got, want) {
		t.Fatalf("TestFiles = %v, want %v", got, want)
	}
}
