package domain

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
)

const (
	StatusAdded    = "A"
	StatusModified = "M"
	StatusDeleted  = "D"
)

const (
	RuleTreeDiff      RuleID = "tree-diff"
	RuleFileUntracked RuleID = "file-untracked"
	RuleCommitMissing RuleID = "commit-missing"
	RuleCommitScope   RuleID = "commit-scope"
	RuleTestMissing   RuleID = "test-missing"
	RuleProofMissing  RuleID = "proof-missing"
)

const (
	ProofPass   = "pass"
	ProofFail   = "fail"
	ProofNotRun = "not run"

	FileAdded    = "added"
	FileModified = "modified"
	FileDeleted  = "deleted"
	FileMissing  = "missing"

	CommitFound   = "found"
	CommitMissing = "missing"
)

const (
	testFuncFormat = "func %s("
	pathSeparator  = "/"
)

var (
	markState = map[string]string{
		MarkNew:     FileAdded,
		MarkChanged: FileModified,
		MarkDeleted: FileDeleted,
	}
	statusState = map[string]string{
		StatusAdded:    FileAdded,
		StatusModified: FileModified,
		StatusDeleted:  FileDeleted,
	}
	ignoredFolders = []string{".harness/", ".kodify/"}
)

type TraceStates struct {
	Files   map[string]string
	Commits map[int]string
	Proofs  map[int]string
}

type FileChange struct {
	Path   string
	Status string
}

type BranchCommit struct {
	SHA     string
	Subject string
	Files   []string
}

type Branch struct {
	Files   []FileChange
	Commits []BranchCommit
	Tests   map[string]string
}

type BranchReader interface {
	Read(ctx context.Context, dir, base string, testFiles []string) (Branch, error)
}

type ProofReader interface {
	Passed(ctx context.Context) (map[int]bool, error)
}

func TestFiles(doc Document) []string {
	var files []string
	for _, t := range doc.Tasks {
		if file, _ := testOf(doc, t); file != "" {
			files = append(files, file)
		}
	}
	return files
}

func testOf(doc Document, t Task) (file, name string) {
	if m := testName.FindStringSubmatch(t.TestFirst); m != nil {
		name = m[1]
	}
	owned := ownedFiles(doc, t)
	best := -1
	for _, token := range ownedPaths(t.TestFirst) {
		candidate, score := token, 0
		if i := slices.IndexFunc(owned, func(o string) bool { return o == token || strings.HasSuffix(o, pathSeparator+token) }); i >= 0 {
			candidate, score = owned[i], 1
		} else if token == "" || strings.ContainsAny(token, " \t") || !strings.Contains(path.Base(token), ".") {
			continue
		}
		if looksLikeTestFile(candidate) {
			score += 2
		}
		if score > best {
			file, best = candidate, score
		}
	}
	return file, name
}

// testStemPattern matches a file stem that names a test: a test or spec word
// between separators, in any case, or a CamelCase Test suffix such as PongTest.
// The suffix is case sensitive so that a stem such as latest is not a test.
var testStemPattern = regexp.MustCompile(`(?i:(?:^|[._-])(?:tests?|spec)(?:$|[._-]))|Tests?$`)

func looksLikeTestFile(p string) bool {
	base := path.Base(p)
	return testStemPattern.MatchString(strings.TrimSuffix(base, path.Ext(base)))
}

func VerifyTrace(doc Document, branch Branch, proofs map[int]bool) []Problem {
	var problems []Problem
	add := func(line int, rule RuleID, format string, args ...any) {
		problems = append(problems, Problem{Line: line, Rule: rule, Message: fmt.Sprintf(format, args...)})
	}
	states := TraceState(doc, &branch, proofs)
	verifyTree(doc, branch.Files, states, add)
	verifyCommits(doc, branch.Commits, states, add)
	verifyTests(doc, branch.Tests, add)
	for _, c := range doc.Criteria {
		if states.Proofs[c.Number] != ProofPass {
			add(c.Line, RuleProofMissing, "criterion %d has no passed proof for the current files", c.Number)
		}
	}
	return problems
}

func TraceState(doc Document, branch *Branch, proofs map[int]bool) TraceStates {
	states := TraceStates{
		Files:   map[string]string{},
		Commits: map[int]string{},
		Proofs:  make(map[int]string, len(doc.Criteria)),
	}
	status := map[string]string{}
	if branch != nil {
		for _, f := range branch.Files {
			if ignored(f.Path) {
				continue
			}
			state, known := statusState[f.Status]
			if !known {
				state = FileModified
			}
			status[f.Path] = state
		}
	}
	for _, tree := range []Tree{doc.CodeTree, doc.TestTree} {
		for _, e := range tree.Entries {
			if !e.IsDir {
				states.Files[e.Path] = fileState(e, branch, status)
			}
		}
	}
	for _, c := range doc.Commits {
		states.Commits[c.Number] = commitState(c, branch)
	}
	for _, c := range doc.Criteria {
		states.Proofs[c.Number] = proofState(proofs, c.Number)
	}
	return states
}

func fileState(e TreeEntry, branch *Branch, status map[string]string) string {
	if branch == nil {
		return markState[e.Mark]
	}
	if state, ok := status[e.Path]; ok {
		return state
	}
	return FileMissing
}

func commitState(c Commit, branch *Branch) string {
	if branch != nil && slices.ContainsFunc(branch.Commits, func(b BranchCommit) bool { return b.Subject == c.Subject }) {
		return CommitFound
	}
	return CommitMissing
}

func proofState(proofs map[int]bool, number int) string {
	passed, ran := proofs[number]
	switch {
	case !ran:
		return ProofNotRun
	case passed:
		return ProofPass
	}
	return ProofFail
}

func ignored(file string) bool {
	return slices.ContainsFunc(ignoredFolders, func(folder string) bool { return strings.HasPrefix(file, folder) })
}

func verifyTree(doc Document, files []FileChange, states TraceStates, add report) {
	listed := map[string]bool{}
	for _, tree := range []Tree{doc.CodeTree, doc.TestTree} {
		for _, e := range tree.Entries {
			if e.IsDir {
				continue
			}
			listed[e.Path] = true
			if got := states.Files[e.Path]; got != markState[e.Mark] {
				add(e.Line, RuleTreeDiff, "%s is marked %s but its state is %s", e.Path, e.Mark, got)
			}
		}
	}
	for _, f := range files {
		if !ignored(f.Path) && !listed[f.Path] {
			add(doc.CodeTree.Line, RuleFileUntracked, "%s is in the diff but in neither final tree", f.Path)
		}
	}
}

func verifyCommits(doc Document, commits []BranchCommit, states TraceStates, add report) {
	for _, planned := range doc.Commits {
		for _, c := range commits {
			if c.Subject != planned.Subject {
				continue
			}
			for _, file := range c.Files {
				if !ignored(file) && !ownedByTasks(doc, planned.Tasks, file) {
					add(planned.Line, RuleCommitScope, "commit %q touches %s, outside the owned files of its tasks", planned.Subject, file)
				}
			}
		}
		if states.Commits[planned.Number] == CommitMissing {
			add(planned.Line, RuleCommitMissing, "commit %q is not in the branch log", planned.Subject)
		}
	}
}

func ownedByTasks(doc Document, numbers []int, file string) bool {
	return slices.ContainsFunc(doc.Tasks, func(t Task) bool {
		return slices.Contains(numbers, t.Number) && owns(t.Owned, file)
	})
}

func verifyTests(doc Document, sources map[string]string, add report) {
	for _, t := range doc.Tasks {
		file, name := testOf(doc, t)
		switch {
		case name == "":
			continue
		case file == "":
			add(t.FieldLines[FieldTestFirst], RuleTestMissing, "task %d: the Test first line names no test file", t.Number)
			continue
		}
		if !strings.Contains(sources[file], fmt.Sprintf(testFuncFormat, name)) {
			add(t.FieldLines[FieldTestFirst], RuleTestMissing, "task %d: %s is not in %s", t.Number, name, file)
		}
	}
}
