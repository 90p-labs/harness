package domain

import (
	"context"
	"fmt"
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
	testFuncFormat = "func %s("
	pathSeparator  = "/"
)

var (
	markStatus = map[string]string{
		MarkNew:     StatusAdded,
		MarkChanged: StatusModified,
		MarkDeleted: StatusDeleted,
	}
	ignoredFolders = []string{".harness/", ".kodify/"}
)

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
	if m := ownedPath.FindStringSubmatch(t.TestFirst); m != nil {
		file = strings.TrimPrefix(m[1], currentDir)
	}
	for _, owned := range ownedFiles(doc, t) {
		if owned == file || strings.HasSuffix(owned, pathSeparator+file) {
			return owned, name
		}
	}
	return file, name
}

func VerifyTrace(doc Document, branch Branch, proofs map[int]bool) []Problem {
	var problems []Problem
	add := func(line int, rule RuleID, format string, args ...any) {
		problems = append(problems, Problem{Line: line, Rule: rule, Message: fmt.Sprintf(format, args...)})
	}
	files := make([]FileChange, 0, len(branch.Files))
	for _, f := range branch.Files {
		if !ignored(f.Path) {
			files = append(files, f)
		}
	}
	verifyTree(doc, files, add)
	verifyCommits(doc, branch.Commits, add)
	verifyTests(doc, branch.Tests, add)
	for _, c := range doc.Criteria {
		if !proofs[c.Number] {
			add(c.Line, RuleProofMissing, "criterion %d has no passed proof for the current files", c.Number)
		}
	}
	return problems
}

func ignored(file string) bool {
	return slices.ContainsFunc(ignoredFolders, func(folder string) bool { return strings.HasPrefix(file, folder) })
}

func verifyTree(doc Document, files []FileChange, add report) {
	status := make(map[string]string, len(files))
	for _, f := range files {
		status[f.Path] = f.Status
	}
	listed := map[string]bool{}
	for _, tree := range []Tree{doc.CodeTree, doc.TestTree} {
		for _, e := range tree.Entries {
			if e.IsDir {
				continue
			}
			listed[e.Path] = true
			if got := status[e.Path]; got != markStatus[e.Mark] {
				add(e.Line, RuleTreeDiff, "%s is marked %s but the diff status is %q", e.Path, e.Mark, got)
			}
		}
	}
	for _, f := range files {
		if !listed[f.Path] {
			add(doc.CodeTree.Line, RuleFileUntracked, "%s is in the diff but in neither final tree", f.Path)
		}
	}
}

func verifyCommits(doc Document, commits []BranchCommit, add report) {
	for _, planned := range doc.Commits {
		found := false
		for _, c := range commits {
			if c.Subject != planned.Subject {
				continue
			}
			found = true
			for _, file := range c.Files {
				if !ignored(file) && !ownedByTasks(doc, planned.Tasks, file) {
					add(planned.Line, RuleCommitScope, "commit %q touches %s, outside the owned files of its tasks", planned.Subject, file)
				}
			}
		}
		if !found {
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
		if name == "" {
			continue
		}
		if !strings.Contains(sources[file], fmt.Sprintf(testFuncFormat, name)) {
			add(t.FieldLines[FieldTestFirst], RuleTestMissing, "task %d: %s is not in %s", t.Number, name, file)
		}
	}
}
