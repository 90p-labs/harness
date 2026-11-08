package domain

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

var testName = regexp.MustCompile("`(Test\\w+)`")

type Links struct {
	Criteria map[int]CriterionLinks
	Tasks    map[int]TaskLinks
	Unlinked []string
}

type CriterionLinks struct {
	Tasks   []int
	Specs   []string
	Proof   string
	Commits []int
	PR      int
	Signal  *SignalRow
}

type TaskLinks struct {
	Files []string
	Test  string
}

type unlinkedItem struct {
	text string
	line int
}

func Trace(doc Document) Links {
	links, _ := trace(doc)
	return links
}

func trace(doc Document) (Links, []unlinkedItem) {
	links := Links{Criteria: map[int]CriterionLinks{}, Tasks: map[int]TaskLinks{}}
	var unlinked []unlinkedItem
	runPR := 0
	for _, pr := range doc.PRs {
		if pr.ThisRun {
			runPR = pr.Number
		}
	}
	for _, c := range doc.Criteria {
		cl := CriterionLinks{Proof: strings.Trim(c.Proof, "`"), PR: runPR}
		for _, t := range doc.Tasks {
			if slices.Contains(t.DoD, c.Number) {
				cl.Tasks = append(cl.Tasks, t.Number)
			}
		}
		for _, s := range doc.Specs {
			if slices.Contains(s.Criteria, c.Number) {
				cl.Specs = append(cl.Specs, s.Title)
			}
		}
		for _, commit := range doc.Commits {
			if slices.ContainsFunc(commit.Tasks, func(n int) bool { return slices.Contains(cl.Tasks, n) }) {
				cl.Commits = append(cl.Commits, commit.Number)
			}
		}
		for _, row := range doc.Signals {
			if row.Criterion == c.Number {
				cl.Signal = &row
				break
			}
		}
		links.Criteria[c.Number] = cl
		if len(cl.Specs) == 0 {
			unlinked = append(unlinked, unlinkedItem{fmt.Sprintf("criterion %d", c.Number), c.Line})
		}
	}
	for _, t := range doc.Tasks {
		tl := TaskLinks{Files: ownedFiles(doc, t)}
		if m := testName.FindStringSubmatch(t.TestFirst); m != nil {
			tl.Test = m[1]
		}
		links.Tasks[t.Number] = tl
		if len(tl.Files) == 0 {
			unlinked = append(unlinked, unlinkedItem{fmt.Sprintf("task %d", t.Number), t.Line})
		}
	}
	for _, item := range unlinked {
		links.Unlinked = append(links.Unlinked, item.text)
	}
	return links, unlinked
}

func ownedFiles(doc Document, t Task) []string {
	var files []string
	for _, tree := range []Tree{doc.CodeTree, doc.TestTree} {
		for _, e := range tree.Entries {
			if !e.IsDir && owns(t.Owned, e.Path) {
				files = append(files, e.Path)
			}
		}
	}
	return files
}

func owns(owned []string, path string) bool {
	return slices.ContainsFunc(owned, func(o string) bool {
		return o == path || (strings.HasSuffix(o, "/") && strings.HasPrefix(path, o))
	})
}

func checkTrace(doc Document, add report) {
	known := make(map[int]bool, len(doc.Criteria))
	for _, c := range doc.Criteria {
		known[c.Number] = true
	}
	for _, s := range doc.Specs {
		if !s.HasCriteriaLine {
			add(s.Line, RuleSpecCriteria, "section %q has no Criteria line", s.Title)
		}
		for _, n := range s.Criteria {
			if !known[n] {
				add(s.Line, RuleSpecCriteria, "section %q names criterion %d, which does not exist", s.Title, n)
			}
		}
	}
	_, unlinked := trace(doc)
	for _, item := range unlinked {
		add(item.line, RuleTraceUnlinked, "%s has no link", item.text)
	}
}
