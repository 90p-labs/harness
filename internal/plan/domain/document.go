package domain

import (
	"slices"
	"strings"
)

const (
	OpenQuestionsNone = "none"
	CommitTypeDocs    = "docs"
)

const (
	HeadingGoal          = "Goal"
	HeadingCriteria      = "Acceptance criteria"
	HeadingTasks         = "Tasks"
	HeadingDecisions     = "Decisions"
	HeadingOpenQuestions = "Open questions"
	HeadingSignals       = "Signals"
	HeadingRegression    = "Regression impact"
	HeadingSpecs         = "Tech specs"
	HeadingCodeTree      = "Final code tree"
	HeadingTestTree      = "Final test tree"
	HeadingCommits       = "Commits"
	HeadingPRs           = "PR sequence"
)

const (
	FieldSide      = "Side"
	FieldOwned     = "Owned files"
	FieldTestFirst = "Test first"
	FieldDoD       = "DoD"
	FieldDependsOn = "Depends on"
	FieldModel     = "Model"
)

const (
	FolderCommand  = "application/command/"
	FolderQuery    = "application/query/"
	FolderHTTP     = "infrastructure/http/"
	FolderConsumer = "infrastructure/consumer/"
	FolderDomain   = "domain/"
)

const (
	MarkNew     = "new"
	MarkChanged = "changed"
	MarkDeleted = "deleted"
)

type Document struct {
	Lines         []string
	Track         string
	Headings      []Heading
	CriteriaTable Table
	Criteria      []Criterion
	Tasks         []Task
	CodeTree      Tree
	TestTree      Tree
	Commits       []Commit
	PRs           []PR
	SignalsTable  Table
	Signals       []SignalRow
	Specs         []Spec
	Diagrams      []Diagram
	Sketches      []Sketch
	OpenQuestions []string
}

func (d Document) QuestionsOpen() bool {
	return len(d.OpenQuestions) == 0 ||
		slices.ContainsFunc(d.OpenQuestions, func(q string) bool { return q != OpenQuestionsNone })
}

func (d Document) ApprovalWaived() bool {
	return len(d.Commits) > 0 && !d.QuestionsOpen() &&
		!slices.ContainsFunc(d.Commits, func(c Commit) bool { return c.Type() != CommitTypeDocs })
}

func (c Commit) Type() string {
	if m := conventional.FindStringSubmatch(c.Subject); m != nil {
		return m[1]
	}
	return ""
}

const SketchExt = ".excalidraw"

type Sketch struct {
	Title string
	Path  string
	Line  int
}

func (d Document) RunPR() PR {
	for _, pr := range d.PRs {
		if pr.ThisRun {
			return pr
		}
	}
	return PR{}
}

func (d Document) Base() string {
	return d.RunPR().Base
}

type Heading struct {
	Level int
	Text  string
	Line  int
}

type Table struct {
	Header []string
	Line   int
}

type Criterion struct {
	Number int
	Text   string
	Proof  string
	Line   int
}

type Task struct {
	Number     int
	Name       string
	Side       string
	Owned      []string
	TestFirst  string
	DoD        []int
	DependsOn  []int
	Model      string
	Line       int
	FieldLines map[string]int
}

type Tree struct {
	Line    int
	Entries []TreeEntry
}

type TreeEntry struct {
	Path  string
	Mark  string
	Line  int
	Depth int
	IsDir bool
}

func (t Tree) Touches(folders ...string) bool {
	for _, e := range t.Entries {
		if e.IsDir {
			continue
		}
		for _, f := range folders {
			if strings.HasPrefix(e.Path, f) || strings.Contains(e.Path, "/"+f) {
				return true
			}
		}
	}
	return false
}

type Commit struct {
	Number  int
	Subject string
	Tasks   []int
	Line    int
}

type PR struct {
	Number  int
	Branch  string
	ThisRun bool
	Base    string
	Line    int
}

type SignalRow struct {
	Criterion int
	Signal    string
	Source    string
	Query     string
	Breach    string
	Window    string
	Panel     string
	Line      int
}

type Spec struct {
	Title           string
	Criteria        []int
	HasCriteriaLine bool
	Line            int
}
