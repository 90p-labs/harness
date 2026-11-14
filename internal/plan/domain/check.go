package domain

import (
	"errors"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
)

var ErrPlanHasProblems = errors.New("plan has problems")

type Report struct {
	Problems []Problem
	Notes    []string
	Track    shareddomain.Track
	Doc      Document
	Program  *Program
}

const (
	SideBackend  = "backend"
	SideFrontend = "frontend"
	SideOther    = "other"
)

const (
	DiagramER       = "erDiagram"
	DiagramSequence = "sequenceDiagram"

	prefixOpenAPI  = "openapi."
	prefixAsyncAPI = "asyncapi."
	infixOpenAPI   = ".openapi."
	suffixProto    = ".proto"
	suffixGraphQL  = ".graphql"

	ContractNames = prefixOpenAPI + "*, " + prefixAsyncAPI + "*, *" + infixOpenAPI + "*, *" + suffixProto + ", *" + suffixGraphQL

	NewDomainFilesForClass = 3
	MaxSentenceWords       = maxSentenceWords
)

var (
	CriteriaHeader = []string{"#", "Criterion", "Proof"}
	SignalsHeader  = []string{"Criterion", "Signal", "Source", "Query", "Breach", "Window", "Panel"}
	TaskSides      = []string{SideBackend, SideFrontend, SideOther}
	TaskFields     = []string{FieldSide, FieldOwned, FieldTestFirst, FieldDoD, FieldDependsOn}
	TopHeadings    = []string{HeadingGoal, HeadingCriteria, HeadingTasks, HeadingDecisions, HeadingReviewRisks, HeadingOpenQuestions, HeadingSpecs}
	FinalHeadings  = []string{HeadingCodeTree, HeadingTestTree, HeadingCommits, HeadingPRs}
	SignalFolders  = []string{FolderCommand, FolderQuery, FolderHTTP, FolderConsumer}

	proofCommand     = regexp.MustCompile("^`[^`]+`$")
	proofPlaceholder = regexp.MustCompile(`(?i)\b(tbd|todo|fixme|xxx)\b|<[a-z](?:[^<>]*[^\s<>])?>|` + "^`\\.\\.\\.`$")
	conventional     = regexp.MustCompile(`^(feat|fix|docs|chore|refactor|test|ci|build|perf|style|revert)(\([^)]+\))?!?: \S`)
)

type CheckInput struct {
	LiveRound     int
	ArchivedPlans []string
	Diagrams      DiagramParser
	Sketches      []string
}

type report func(line int, rule RuleID, format string, args ...any)

func Check(doc Document, in CheckInput) []Problem {
	var problems []Problem
	add := reporter(&problems)
	checkHeadings(doc, add)
	checkCriteria(doc, add)
	checkTasks(doc, add)
	checkSeams(doc, add)
	checkTrees(doc, add)
	checkCommits(doc, add)
	checkPRs(doc, add)
	checkSignals(doc, add)
	checkRegression(doc, in, add)
	checkDiagrams(doc, in, add)
	checkRequiredDiagrams(doc, in, add)
	checkSentences(doc, add)
	checkTrace(doc, add)
	checkOpenQuestions(doc, add)
	checkReviewRisks(doc, add)
	checkThreatModel(doc, add)
	checkPlanSize(doc, add)
	SortProblems(problems)
	return problems
}

func reporter(problems *[]Problem) report {
	return func(line int, rule RuleID, format string, args ...any) {
		*problems = append(*problems, Problem{Line: line, Rule: rule, Message: fmt.Sprintf(format, args...)})
	}
}

func SortProblems(problems []Problem) {
	slices.SortStableFunc(problems, func(a, b Problem) int {
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		return strings.Compare(string(a.Rule), string(b.Rule))
	})
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
	for _, text := range TopHeadings {
		require(2, text)
	}
	for _, text := range FinalHeadings {
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
	tail := subs[max(0, len(subs)-len(FinalHeadings)):]
	inOrder := slices.EqualFunc(tail, FinalHeadings, func(h Heading, text string) bool { return h.Text == text })
	if !inOrder {
		line := specs
		if len(tail) > 0 {
			line = tail[0].Line
		}
		add(line, RuleHeadingMissing, "the tech specs must end with %s, in this order", strings.Join(FinalHeadings, ", "))
	}
}

func checkOpenQuestions(doc Document, add report) {
	line := headingLine(doc, 2, HeadingOpenQuestions)
	if line == 0 {
		return
	}
	hasNone := slices.Contains(doc.OpenQuestions, OpenQuestionsNone)
	hasQuestion := slices.ContainsFunc(doc.OpenQuestions, func(q string) bool { return q != OpenQuestionsNone })
	switch {
	case len(doc.OpenQuestions) == 0:
		add(line, RuleOpenQuestions, "write `- %s` or one question on each line", OpenQuestionsNone)
	case hasNone && hasQuestion:
		add(line, RuleOpenQuestions, "`- %s` stands beside a question: delete one of them", OpenQuestionsNone)
	}
}

func checkCriteria(doc Document, add report) {
	if doc.CriteriaTable.Header != nil && !slices.Equal(doc.CriteriaTable.Header, CriteriaHeader) {
		add(doc.CriteriaTable.Line, RuleCriteriaTable, "header must be | %s |", strings.Join(CriteriaHeader, " | "))
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
		for _, f := range TaskFields {
			if _, ok := t.FieldLines[f]; !ok {
				add(t.Line, RuleTaskField, "task %d has no %s: line", t.Number, f)
			}
		}
		if _, ok := t.FieldLines[FieldSide]; ok && !slices.Contains(TaskSides, t.Side) {
			add(t.FieldLines[FieldSide], RuleTaskField, "Side must be %s", strings.Join(TaskSides, ", "))
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
	checkCommitSplit(doc, add)
}

func checkCommitSplit(doc Document, add report) {
	var tests []Commit
	hasDocs, hasCode := false, false
	for _, c := range doc.Commits {
		switch c.Type() {
		case "test":
			tests = append(tests, c)
		case CommitTypeDocs:
			hasDocs = true
		case "feat", "fix":
			hasCode = true
		}
	}
	if !hasDocs || hasCode {
		return
	}
	for _, c := range tests {
		add(c.Line, RuleCommitSplit, "commit %q is a test commit in a docs change, which verify --commits rejects when it fails. Drop it and prove the docs with a proof command", c.Subject)
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

func signalsHeaderRow() string {
	return "| " + strings.Join(SignalsHeader, " | ") + " |"
}

func checkSignals(doc Document, add report) {
	heading := headingLine(doc, 2, HeadingSignals)
	if heading == 0 {
		if doc.CodeTree.Touches(SignalFolders...) {
			add(lineOr(headingLine(doc, 3, HeadingCodeTree), 1), RuleSignalsMissing, "the code tree adds a command, query, endpoint or consumer, so the plan needs ## Signals. Add ## Signals with the header %s", signalsHeaderRow())
		}
		return
	}
	if len(doc.Signals) == 0 {
		add(heading, RuleSignalsTable, "## Signals needs a table with one row or more. Header: %s", signalsHeaderRow())
	}
	if doc.SignalsTable.Header != nil && !slices.Equal(doc.SignalsTable.Header, SignalsHeader) {
		add(doc.SignalsTable.Line, RuleSignalsTable, "header must be | %s |", strings.Join(SignalsHeader, " | "))
	}
	for _, row := range doc.Signals {
		if _, err := shareddomain.ParseBreach(row.Breach); err != nil {
			add(row.Line, RuleSignalsTable, "breach %q must be an operator (>, >=, <, <=) and a number. Example: \"> 0.01\"", row.Breach)
		}
		if _, err := time.ParseDuration(row.Window); err != nil {
			add(row.Line, RuleSignalsTable, "window %q is not a Go duration. Example: \"30m\" or \"24h\"", row.Window)
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

func hasDiagram(doc Document, kind string) bool {
	return slices.ContainsFunc(doc.Diagrams, func(d Diagram) bool { return d.Kind == kind })
}

func isContract(name string) bool {
	return strings.HasPrefix(name, prefixOpenAPI) || strings.HasPrefix(name, prefixAsyncAPI) ||
		strings.Contains(name, infixOpenAPI) || strings.HasSuffix(name, suffixProto) || strings.HasSuffix(name, suffixGraphQL)
}

func requiredDiagrams(e TreeEntry) []string {
	name := path.Base(e.Path)
	var kinds []string
	if strings.Contains(e.Path, FolderMigrations) || strings.HasSuffix(name, SuffixSQL) {
		kinds = append(kinds, DiagramER)
	}
	if isContract(name) {
		kinds = append(kinds, DiagramClass)
	}
	if e.Mark == MarkNew && (strings.Contains(e.Path, FolderHTTP) || strings.Contains(e.Path, FolderConsumer)) {
		kinds = append(kinds, DiagramSequence)
	}
	return kinds
}

func checkRequiredDiagrams(doc Document, in CheckInput, add report) {
	newDomainFiles := 0
	for _, e := range doc.CodeTree.Entries {
		if e.IsDir {
			continue
		}
		for _, kind := range requiredDiagrams(e) {
			if !hasDiagram(doc, kind) {
				add(e.Line, RuleDiagramRequired, "%s needs a %s", e.Path, kind)
			}
		}
		if e.Mark == MarkNew && strings.Contains(e.Path, FolderDomain) {
			newDomainFiles++
			if newDomainFiles == NewDomainFilesForClass && !hasDiagram(doc, DiagramClass) {
				add(e.Line, RuleDiagramRequired, "%d new files in %s need a %s", newDomainFiles, FolderDomain, DiagramClass)
			}
		}
	}
	for _, t := range doc.Tasks {
		if t.Side == SideFrontend && len(doc.Sketches) == 0 {
			add(t.Line, RuleDiagramRequired, "task %d is frontend and needs a sketch link", t.Number)
		}
	}
	for _, s := range doc.Sketches {
		if !slices.Contains(in.Sketches, s.Path) {
			add(s.Line, RuleDiagramRequired, "sketch %s is not a file in the plan folder", s.Path)
		}
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

func checkSeams(doc Document, add report) {
	for _, t := range doc.Tasks {
		file, name := testOf(doc, t)
		if name == "" && !(file != "" && looksLikeTestFile(file)) {
			continue
		}
		line := t.FieldLines[FieldTestFirst]
		var names []string
		for _, m := range testName.FindAllStringSubmatch(t.TestFirst, -1) {
			names = append(names, m[1])
		}
		if !slices.ContainsFunc(t.Seams, func(s string) bool { return !slices.Contains(names, s) && !looksLikeTestFile(s) }) {
			add(line, RuleSeamMissing, "task %d names a test and no seam apart from the test: write %s", t.Number, FormSeam)
		}
	}
}
