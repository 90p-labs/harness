package domain

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

var ErrPlanHasProblems = errors.New("plan has problems")

type Report struct {
	Problems []Problem
	Notes    []string
	Track    shareddomain.Track
}

const (
	SideBackend  = "backend"
	SideFrontend = "frontend"
	SideOther    = "other"
)

var (
	criteriaHeader = []string{"#", "Criterion", "Proof"}
	signalsHeader  = []string{"Criterion", "Metric", "Panel", "Alert", "Threshold", "Window"}
	taskSides      = []string{SideBackend, SideFrontend, SideOther}
	taskFields     = []string{FieldSide, FieldOwned, FieldTestFirst, FieldDoD, FieldDependsOn}
	topHeadings    = []string{HeadingGoal, HeadingCriteria, HeadingTasks, HeadingDecisions, HeadingSpecs}
	finalHeadings  = []string{HeadingCodeTree, HeadingTestTree, HeadingCommits, HeadingPRs}

	proofCommand     = regexp.MustCompile("^`[^`]+`$")
	proofPlaceholder = regexp.MustCompile(`(?i)\b(tbd|todo|fixme|xxx)\b|<[a-z](?:[^<>]*[^\s<>])?>|` + "^`\\.\\.\\.`$")
	conventional     = regexp.MustCompile(`^(feat|fix|docs|chore|refactor|test|ci|build|perf|style|revert)(\([^)]+\))?!?: \S`)
)

type CheckInput struct {
	LiveRound     int
	ArchivedPlans []string
	Diagrams      DiagramParser
}

type report func(line int, rule RuleID, format string, args ...any)

func Check(doc Document, in CheckInput) []Problem {
	var problems []Problem
	add := func(line int, rule RuleID, format string, args ...any) {
		problems = append(problems, Problem{Line: line, Rule: rule, Message: fmt.Sprintf(format, args...)})
	}
	checkHeadings(doc, add)
	checkCriteria(doc, add)
	checkTasks(doc, add)
	checkTrees(doc, add)
	checkCommits(doc, add)
	checkPRs(doc, add)
	checkSignals(doc, add)
	checkRegression(doc, in, add)
	checkDiagrams(doc, in, add)
	checkSentences(doc, add)
	checkTrace(doc, add)
	slices.SortStableFunc(problems, func(a, b Problem) int {
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(string(a.Rule), string(b.Rule))
	})
	return problems
}

func headingLine(doc Document, level int, text string) int {
	for _, h := range doc.Headings {
		if h.Level == level && h.Text == text {
			return h.Line
		}
	}
	return 0
}

func lineOr(line, fallback int) int {
	if line == 0 {
		return fallback
	}
	return line
}

func checkHeadings(doc Document, add report) {
	missing := false
	require := func(level int, text string) {
		if headingLine(doc, level, text) == 0 {
			missing = true
			add(1, RuleHeadingMissing, "heading %s is missing", strings.Repeat("#", level)+" "+text)
		}
	}
	for _, text := range topHeadings {
		require(2, text)
	}
	for _, text := range finalHeadings {
		require(3, text)
	}
	specs := headingLine(doc, 2, HeadingSpecs)
	if missing || specs == 0 {
		return
	}
	var subs []Heading
	for _, h := range doc.Headings {
		if h.Line > specs && h.Level == 3 {
			subs = append(subs, h)
		}
	}
	tail := subs[max(0, len(subs)-len(finalHeadings)):]
	inOrder := slices.EqualFunc(tail, finalHeadings, func(h Heading, text string) bool { return h.Text == text })
	if !inOrder {
		line := specs
		if len(tail) > 0 {
			line = tail[0].Line
		}
		add(line, RuleHeadingMissing, "the tech specs must end with %s, in this order", strings.Join(finalHeadings, ", "))
	}
}

func checkCriteria(doc Document, add report) {
	if doc.CriteriaTable.Header != nil && !slices.Equal(doc.CriteriaTable.Header, criteriaHeader) {
		add(doc.CriteriaTable.Line, RuleCriteriaTable, "header must be | %s |", strings.Join(criteriaHeader, " | "))
	}
	for i, c := range doc.Criteria {
		if c.Number != i+1 {
			add(c.Line, RuleCriteriaTable, "criterion number is %d, want %d", c.Number, i+1)
		}
		switch {
		case !proofCommand.MatchString(c.Proof):
			add(c.Line, RuleCriteriaTable, "proof must be a command in backticks")
		case proofPlaceholder.MatchString(c.Proof):
			add(c.Line, RuleCriteriaTable, "proof %s is a placeholder", c.Proof)
		}
	}
}

func checkTasks(doc Document, add report) {
	criteria := make(map[int]bool, len(doc.Criteria))
	for _, c := range doc.Criteria {
		criteria[c.Number] = true
	}
	tasks := make(map[int]bool, len(doc.Tasks))
	for _, t := range doc.Tasks {
		if tasks[t.Number] {
			add(t.Line, RuleTaskField, "task number %d is used twice", t.Number)
		}
		tasks[t.Number] = true
	}
	owned := map[int]bool{}
	for _, t := range doc.Tasks {
		for _, f := range taskFields {
			if _, ok := t.FieldLines[f]; !ok {
				add(t.Line, RuleTaskField, "task %d has no %s: line", t.Number, f)
			}
		}
		if _, ok := t.FieldLines[FieldSide]; ok && !slices.Contains(taskSides, t.Side) {
			add(t.FieldLines[FieldSide], RuleTaskField, "Side must be %s", strings.Join(taskSides, ", "))
		}
		for _, n := range t.DoD {
			owned[n] = true
			if !criteria[n] {
				add(t.FieldLines[FieldDoD], RuleDoDUnknown, "task %d names criterion %d, which does not exist", t.Number, n)
			}
		}
		for _, d := range t.DependsOn {
			if !tasks[d] {
				add(t.FieldLines[FieldDependsOn], RuleTaskDepends, "task %d depends on task %d, which does not exist", t.Number, d)
			}
		}
	}
	for _, c := range doc.Criteria {
		if !owned[c.Number] {
			add(c.Line, RuleCriterionUnowned, "criterion %d is in no DoD line", c.Number)
		}
	}
	var cycle CycleError
	if _, err := Waves(doc.Tasks); errors.As(err, &cycle) {
		first := doc.Tasks[slices.IndexFunc(doc.Tasks, func(t Task) bool { return t.Number == cycle.Tasks[0] })]
		add(lineOr(first.FieldLines[FieldDependsOn], first.Line), RuleTaskDepends, "%s", cycle)
	}
}

func checkTrees(doc Document, add report) {
	for _, tree := range []Tree{doc.CodeTree, doc.TestTree} {
		for _, e := range tree.Entries {
			if e.IsDir {
				continue
			}
			if !slices.ContainsFunc(doc.Tasks, func(t Task) bool { return owns(t.Owned, e.Path) }) {
				add(e.Line, RuleTreeUnowned, "no task owns %s", e.Path)
			}
		}
	}
}

func checkCommits(doc Document, add report) {
	tasks := make(map[int]bool, len(doc.Tasks))
	for _, t := range doc.Tasks {
		tasks[t.Number] = true
	}
	committed := map[int]bool{}
	for _, c := range doc.Commits {
		if !conventional.MatchString(c.Subject) {
			add(c.Line, RuleCommitFormat, "subject %q is not a conventional commit", c.Subject)
		}
		for _, n := range c.Tasks {
			committed[n] = true
			if !tasks[n] {
				add(c.Line, RuleCommitFormat, "commit names task %d, which does not exist", n)
			}
		}
	}
	for _, t := range doc.Tasks {
		if !committed[t.Number] {
			add(t.Line, RuleTaskUncommitted, "task %d is in no commit", t.Number)
		}
	}
}

func checkPRs(doc Document, add report) {
	heading := headingLine(doc, 3, HeadingPRs)
	if heading == 0 {
		return
	}
	var runPRs []PR
	for _, pr := range doc.PRs {
		if pr.ThisRun {
			runPRs = append(runPRs, pr)
		}
	}
	switch {
	case len(runPRs) == 0:
		add(heading, RulePRSequence, "no PR is marked (this run)")
	case len(runPRs) > 1:
		add(runPRs[1].Line, RulePRSequence, "%d PRs are marked (this run), want 1", len(runPRs))
	}
}

func checkSignals(doc Document, add report) {
	heading := headingLine(doc, 2, HeadingSignals)
	if heading == 0 {
		if doc.CodeTree.Touches(FolderCommand, FolderQuery, FolderHTTP, FolderConsumer) {
			add(lineOr(headingLine(doc, 3, HeadingCodeTree), 1), RuleSignalsMissing, "the code tree adds a command, query, endpoint or consumer, so the plan needs ## Signals")
		}
		return
	}
	if len(doc.Signals) == 0 {
		add(heading, RuleSignalsTable, "## Signals needs a table with one row or more")
	}
	if doc.SignalsTable.Header != nil && !slices.Equal(doc.SignalsTable.Header, signalsHeader) {
		add(doc.SignalsTable.Line, RuleSignalsTable, "header must be | %s |", strings.Join(signalsHeader, " | "))
	}
	for _, row := range doc.Signals {
		if _, err := time.ParseDuration(row.Window); err != nil {
			add(row.Line, RuleSignalsTable, "window %q is not a Go duration", row.Window)
		}
	}
}

func checkRegression(doc Document, in CheckInput, add report) {
	if in.LiveRound <= 0 {
		return
	}
	heading := headingLine(doc, 2, HeadingRegression)
	if heading == 0 {
		add(1, RuleRegressionImpact, "live round %d needs ## Regression impact", in.LiveRound)
		return
	}
	end := len(doc.Lines)
	for _, h := range doc.Headings {
		if h.Level == 2 && h.Line > heading {
			end = h.Line - 1
			break
		}
	}
	section := strings.Join(doc.Lines[heading:end], "\n")
	if !slices.ContainsFunc(in.ArchivedPlans, func(p string) bool { return strings.Contains(section, p) }) {
		add(heading, RuleRegressionImpact, "the section names no archived plan file")
	}
}

func checkDiagrams(doc Document, in CheckInput, add report) {
	for _, d := range doc.Diagrams {
		for _, problem := range shareddomain.CheckMarks(d.Body) {
			add(d.Line, RuleDiagram, "%s", problem)
		}
		if in.Diagrams == nil || !d.Checked() {
			continue
		}
		if err := in.Diagrams.Parse(d.Kind, d.Body); err != nil {
			add(d.Line, RuleDiagram, "%s", err)
		}
	}
}
