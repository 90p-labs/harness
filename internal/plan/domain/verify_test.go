package domain_test

import (
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
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
	typeChange    = "T"
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

	t.Run("a test file outside the owned files beats a dotted identifier before it", func(t *testing.T) {
		doc := readFixture(t, "verify_clean.md")
		doc.Tasks[0].TestFirst = "`domain.Pong` is missing, so `TestPong` in `elsewhere_test.go` fails."
		b := cleanBranch()
		b.Tests["elsewhere_test.go"] = "package domain_test\n\nfunc TestPong(t *testing.T) {}\n"

		got := domain.VerifyTrace(doc, b, allPass)

		if len(got) != 0 {
			t.Fatalf("problems = %v, want none", got)
		}
	})

	t.Run("a test file outside the owned files beats a dotted type that only holds test", func(t *testing.T) {
		doc := readFixture(t, "verify_clean.md")
		doc.Tasks[0].TestFirst = "`testing.T` is unused, so `TestPong` in `elsewhere_test.go` fails."
		b := cleanBranch()
		b.Tests["elsewhere_test.go"] = "package domain_test\n\nfunc TestPong(t *testing.T) {}\n"

		got := domain.VerifyTrace(doc, b, allPass)

		if len(got) != 0 {
			t.Fatalf("problems = %v, want none", got)
		}
		if files := domain.TestFiles(doc); !slices.Contains(files, "elsewhere_test.go") || slices.Contains(files, "testing.T") {
			t.Fatalf("TestFiles = %v, want elsewhere_test.go and not testing.T", files)
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

func TestTraceState(t *testing.T) {
	doc := readFixture(t, "verify_clean.md")

	t.Run("a branch gives the diff state of each file and the log state of each commit", func(t *testing.T) {
		b := cleanBranch()
		b.Files[0].Status = domain.StatusModified
		b.Files = slices.DeleteFunc(b.Files, func(f domain.FileChange) bool { return f.Path == moduleFile })
		for i, f := range b.Files {
			if f.Path == handlerTest {
				b.Files[i].Status = typeChange
			}
		}
		b.Commits = b.Commits[:1]

		got := domain.TraceState(doc, &b, map[int]bool{1: true, 2: false})

		wantFiles := map[string]string{
			pongFile:     domain.FileModified,
			pongTestFile: domain.FileAdded,
			legacyFile:   domain.FileDeleted,
			handlerFile:  domain.FileAdded,
			handlerTest:  domain.FileModified,
			moduleFile:   domain.FileMissing,
		}
		if !maps.Equal(got.Files, wantFiles) {
			t.Errorf("files = %v, want %v", got.Files, wantFiles)
		}
		wantCommits := map[int]string{1: domain.CommitFound, 2: domain.CommitMissing}
		if !maps.Equal(got.Commits, wantCommits) {
			t.Errorf("commits = %v, want %v", got.Commits, wantCommits)
		}
		wantProofs := map[int]string{1: domain.ProofPass, 2: domain.ProofFail}
		if !maps.Equal(got.Proofs, wantProofs) {
			t.Errorf("proofs = %v, want %v", got.Proofs, wantProofs)
		}
	})

	t.Run("no branch gives the plan mark of each file and a missing commit", func(t *testing.T) {
		got := domain.TraceState(doc, nil, nil)

		wantFiles := map[string]string{
			pongFile:     domain.FileAdded,
			pongTestFile: domain.FileAdded,
			legacyFile:   domain.FileDeleted,
			handlerFile:  domain.FileAdded,
			handlerTest:  domain.FileAdded,
			moduleFile:   domain.FileModified,
		}
		if !maps.Equal(got.Files, wantFiles) {
			t.Errorf("files = %v, want %v", got.Files, wantFiles)
		}
		wantCommits := map[int]string{1: domain.CommitMissing, 2: domain.CommitMissing}
		if !maps.Equal(got.Commits, wantCommits) {
			t.Errorf("commits = %v, want %v", got.Commits, wantCommits)
		}
		wantProofs := map[int]string{1: domain.ProofNotRun, 2: domain.ProofNotRun}
		if !maps.Equal(got.Proofs, wantProofs) {
			t.Errorf("proofs = %v, want %v", got.Proofs, wantProofs)
		}
	})

	t.Run("VerifyTrace reports the same file and commit states", func(t *testing.T) {
		b := brokenBranch()
		b.Commits = b.Commits[:1]

		states := domain.TraceState(doc, &b, nil)
		problems := domain.VerifyTrace(doc, b, nil)

		if states.Files[pongFile] != domain.FileModified || states.Commits[2] != domain.CommitMissing {
			t.Fatalf("states = %+v", states)
		}
		for _, rule := range []domain.RuleID{domain.RuleTreeDiff, domain.RuleCommitMissing} {
			if !slices.Contains(problemRules(problems), rule) {
				t.Errorf("rules = %v, want %s", problemRules(problems), rule)
			}
		}
	})
}

func TestTestFirstPath(t *testing.T) {
	const cleanLine = "- Test first: `pong_test.go`: `TestPong` fails, because `Pong` does not exist."
	raw, err := os.ReadFile("testdata/verify_clean.md")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"name before file":              "- Test first: add `TestPong` in `pong_test.go`. It fails, because `Pong` does not exist.",
		"name then Pong then file":      "- Test first: add `TestPong` for `Pong` in `pong_test.go`. It fails.",
		"dot slash file":                "- Test first: `./pong_test.go`: `TestPong` fails, because `Pong` does not exist.",
		"command before file":           "- Test first: run `go test ./internal/ping/domain -run TestPong` on `pong_test.go`: `TestPong` fails.",
		"dotted identifier before file": "- Test first: `domain.Pong` is missing, so `TestPong` in `pong_test.go` fails.",
		"owned source before its test":  "- Test first: `pong.go` lacks `Pong`, so `TestPong` in `pong_test.go` fails.",
	}
	// The cases do not use t.Run: the criterion proof greps `--- PASS: TestTestFirstPath`,
	// which a subtest PASS line would also match.
	for name, line := range cases {
		text := strings.Replace(string(raw), cleanLine, line, 1)
		if text == string(raw) {
			t.Fatalf("%s: the task 1 line is not in the fixture", name)
		}
		doc, problems := domain.Parse(text)
		if len(problems) != 0 {
			t.Fatalf("%s: parse problems: %v", name, problems)
		}

		if got, want := domain.TestFiles(doc), []string{pongTestFile, handlerTest}; !slices.Equal(got, want) {
			t.Errorf("%s: TestFiles = %v, want %v", name, got, want)
		}
		got := domain.VerifyTrace(doc, cleanBranch(), map[int]bool{1: true, 2: true})
		if slices.Contains(problemRules(got), domain.RuleTestMissing) {
			t.Errorf("%s: problems = %v, want no test-missing", name, got)
		}
	}
}

func TestTestFirstNamesNoFile(t *testing.T) {
	const cleanLine = "- Test first: `pong_test.go`: `TestPong` fails, because `Pong` does not exist."
	raw, err := os.ReadFile("testdata/verify_clean.md")
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Replace(string(raw), cleanLine, "- Test first: `TestPong` fails, because `Pong` does not exist.", 1)
	if text == string(raw) {
		t.Fatal("the task 1 line is not in the fixture")
	}
	doc, problems := domain.Parse(text)
	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}

	got := domain.VerifyTrace(doc, cleanBranch(), map[int]bool{1: true, 2: true})

	for _, p := range got {
		if p.Rule == domain.RuleTestMissing && strings.Contains(p.Message, "names no test file") {
			return
		}
	}
	t.Errorf("problems = %v, want a test-missing problem that names no test file", got)
}

func TestTestFiles(t *testing.T) {
	got := domain.TestFiles(readFixture(t, "verify_clean.md"))

	if want := []string{pongTestFile, handlerTest}; !slices.Equal(got, want) {
		t.Fatalf("TestFiles = %v, want %v", got, want)
	}
}
