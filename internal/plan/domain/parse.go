package domain

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	shared "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	TrackPrefix    = "Track:"
	CriteriaKey    = "Criteria:"
	FenceTree      = "tree"
	TreeLevelWidth = 4
	TreeMarks      = MarkNew + ", " + MarkChanged + " or " + MarkDeleted

	FormTaskHeading  = "n. Name"
	FormDoD          = "criterion n or criteria a, b and c"
	FormDependsOn    = "none, task n or tasks a, b and c"
	FormSpecCriteria = "a list such as 1, 2 and 3 or 4 to 9"
	FormCommit       = "n. `type: subject` (task n)"
	FormPR           = "n. `type/slug` (this run), based on `base`: text"
)

const (
	fenceMark      = "```"
	fenceMermaid   = "mermaid"
	dependsNone    = "none"
	wordTo         = "to"
	sentenceStop   = "."
	currentDir     = "./"
	frontMatter    = "---"
	mermaidComment = "%%"
	trailingSpace  = " \t\r"
	bulletMark     = "- "
)

const (
	treeBar        = "│   "
	treeBlank      = "    "
	treeTee        = "├── "
	treeLast       = "└── "
	treeIndentRune = " │├└─"
)

var (
	numberedLine   = regexp.MustCompile(`^\d+\.\s`)
	taskHeading    = regexp.MustCompile(`^(\d+)\.\s+(.+)$`)
	commitLine     = regexp.MustCompile("^(\\d+)\\.\\s+`([^`]*)`\\s+\\((.+)\\)\\s*$")
	prLine         = regexp.MustCompile("^(\\d+)\\.\\s+`([^`]+)`( \\(this run\\))?(?:, based on `([^`]+)`)?:\\s*(.*)$")
	sketchLink     = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+` + regexp.QuoteMeta(SketchExt) + `)\)`)
	ownedPath      = regexp.MustCompile("`([^`]+)`")
	leadingNumber  = regexp.MustCompile(`^\d+`)
	followingNum   = regexp.MustCompile(`^(,\s*and\s+|,\s*|\s+and\s+|\s+to\s+)(\d+)`)
	commitTaskNoun = []string{"task ", "tasks "}
)

const (
	criteriaCells = 3
	signalsCells  = 7
	maxRangeSpan  = 1000
)

type fence struct {
	info string
	line int
	body []string
}

type parser struct {
	doc      Document
	problems []Problem
	section  string
	sub      string
	task     int
	fence    *fence
}

func Parse(text string) (Document, []Problem) {
	p := &parser{doc: Document{Lines: strings.Split(text, "\n")}, task: -1}
	for i, raw := range p.doc.Lines {
		p.line(strings.TrimRight(raw, trailingSpace), i+1)
	}
	return p.doc, p.problems
}

func (p *parser) problem(line int, rule RuleID, format string, args ...any) {
	p.problems = append(p.problems, Problem{Line: line, Rule: rule, Message: fmt.Sprintf(format, args...)})
}

func (p *parser) line(line string, n int) {
	switch {
	case p.fence != nil && shared.IsFenceClose(line):
		p.closeFence()
	case p.fence != nil:
		p.fence.body = append(p.fence.body, line)
	case shared.IsFenceOpen(line):
		p.fence = &fence{info: strings.TrimSpace(strings.TrimPrefix(line, fenceMark)), line: n}
	case strings.HasPrefix(line, "### "):
		p.heading(3, strings.TrimPrefix(line, "### "), n)
	case strings.HasPrefix(line, "## "):
		p.heading(2, strings.TrimPrefix(line, "## "), n)
	default:
		p.content(line, n)
	}
}

func (p *parser) heading(level int, text string, n int) {
	p.doc.Headings = append(p.doc.Headings, Heading{Level: level, Text: text, Line: n})
	if level == 2 {
		if text == HeadingReproduction {
			p.reproductionHeading(n)
		}
		p.section, p.sub, p.task = text, "", -1
		return
	}
	p.sub, p.task = text, -1
	switch {
	case p.section == HeadingTasks:
		p.startTask(text, n)
	case p.section == HeadingSpecs && !isFinalHeading(text):
		p.doc.Specs = append(p.doc.Specs, Spec{Title: text, Line: n})
	}
}

func isFinalHeading(text string) bool {
	switch text {
	case HeadingCodeTree, HeadingTestTree, HeadingCommits, HeadingPRs:
		return true
	}
	return false
}

func (p *parser) content(line string, n int) {
	for _, m := range sketchLink.FindAllStringSubmatch(shared.MaskCodeSpans(line), -1) {
		p.doc.Sketches = append(p.doc.Sketches, Sketch{Title: m[1], Path: path.Clean(m[2]), Line: n})
	}
	switch p.section {
	case HeadingGoal:
		if v, ok := strings.CutPrefix(line, TrackPrefix); ok {
			p.doc.Track = strings.ToLower(strings.TrimSpace(v))
		}
		if v, ok := strings.CutPrefix(line, CausePrefix); ok {
			p.doc.Cause, p.doc.CauseLine = strings.ToLower(strings.TrimSpace(v)), n
		}
	case HeadingReproduction:
		p.reproductionLine(line, n)
	case HeadingCriteria:
		p.row(&p.doc.CriteriaTable, line, n, RuleCriteriaTable, criteriaCells, p.addCriterion)
	case HeadingSignals:
		p.row(&p.doc.SignalsTable, line, n, RuleSignalsTable, signalsCells, p.addSignal)
	case HeadingTasks:
		p.taskField(line, n)
	case HeadingSpecs:
		p.specsContent(line, n)
	case HeadingOpenQuestions:
		p.openQuestion(line, n)
	case HeadingReviewRisks:
		p.reviewRisk(line, n)
	case HeadingThreatModel:
		if bullet, ok := strings.CutPrefix(line, bulletMark); ok {
			p.doc.ThreatModel = append(p.doc.ThreatModel, strings.TrimSpace(bullet))
		}
	}
}

func (p *parser) reviewRisk(line string, n int) {
	if line == "" {
		return
	}
	risk, ok := strings.CutPrefix(line, bulletMark)
	if !ok {
		p.problem(n, RuleRiskUnanchored, "line must be `- %s` or `- risk`", RisksNone)
		return
	}
	p.doc.ReviewRisks = append(p.doc.ReviewRisks, Risk{Text: strings.TrimSpace(risk), Line: n})
}

func (p *parser) openQuestion(line string, n int) {
	if line == "" {
		return
	}
	question, ok := strings.CutPrefix(line, bulletMark)
	if !ok {
		p.problem(n, RuleOpenQuestions, "line must be `- %s` or `- question`", OpenQuestionsNone)
		return
	}
	p.doc.OpenQuestions = append(p.doc.OpenQuestions, strings.TrimSpace(question))
}

func (p *parser) specsContent(line string, n int) {
	switch {
	case p.sub == HeadingCommits:
		p.commit(line, n)
	case p.sub == HeadingPRs:
		p.pr(line, n)
	case p.sub != "" && !isFinalHeading(p.sub):
		p.specLine(line, n)
	}
}

func (p *parser) row(t *Table, line string, n int, rule RuleID, width int, add func(c []string, n int)) {
	if !strings.HasPrefix(line, "|") {
		return
	}
	c := shared.TableCells(line)
	switch {
	case t.Header == nil:
		t.Header, t.Line = c, n
	case shared.IsTableSeparator(c):
	case len(c) != width:
		p.problem(n, rule, "row has %d cells, want %d. Give one cell for each column of the header | %s |",
			len(c), width, strings.Join(t.Header, " | "))
	default:
		add(c, n)
	}
}

func (p *parser) addCriterion(c []string, n int) {
	number, err := strconv.Atoi(c[0])
	if err != nil {
		p.problem(n, RuleCriteriaTable, "criterion number %q is not a number", c[0])
		return
	}
	p.doc.Criteria = append(p.doc.Criteria, Criterion{Number: number, Text: c[1], Proof: c[2], Line: n})
}

func (p *parser) addSignal(c []string, n int) {
	criterion, err := strconv.Atoi(c[0])
	if err != nil {
		p.problem(n, RuleSignalsTable, "signal criterion %q is not a number. Example: \"1\"", c[0])
		return
	}
	p.doc.Signals = append(p.doc.Signals, SignalRow{
		Criterion: criterion, Signal: c[1], Source: c[2], Query: strings.Trim(c[3], "`"),
		Breach: strings.Trim(c[4], "`"), Window: strings.Trim(c[5], "`"), Panel: c[6], Line: n,
	})
}

func (p *parser) startTask(text string, n int) {
	m := taskHeading.FindStringSubmatch(text)
	if m == nil {
		p.problem(n, RuleTaskField, "task heading %q must be %s", text, FormTaskHeading)
		return
	}
	number, _ := strconv.Atoi(m[1])
	p.doc.Tasks = append(p.doc.Tasks, Task{Number: number, Name: m[2], Line: n, FieldLines: map[string]int{}})
	p.task = len(p.doc.Tasks) - 1
}

func ownedPaths(value string) []string {
	var paths []string
	for _, m := range ownedPath.FindAllStringSubmatch(value, -1) {
		paths = append(paths, strings.TrimPrefix(m[1], currentDir))
	}
	return paths
}

func (p *parser) taskField(line string, n int) {
	bullet, ok := strings.CutPrefix(line, bulletMark)
	if p.task < 0 || !ok {
		return
	}
	key, value, ok := strings.Cut(bullet, ":")
	if !ok {
		return
	}
	value = strings.TrimSpace(value)
	t := &p.doc.Tasks[p.task]
	switch key {
	case FieldSide:
		t.Side = value
	case FieldOwned:
		t.Owned = append(t.Owned, ownedPaths(value)...)
	case FieldTestFirst:
		t.TestFirst = value
		t.Seams = seamNames(value)
	case FieldModel:
		t.Model = value
	case FieldDoD:
		numbers, _, ok := parseNumbers(value, "criterion ", "criteria ")
		if !ok {
			p.problem(n, RuleTaskField, "DoD must start with %s", FormDoD)
		}
		t.DoD = numbers
	case FieldDependsOn:
		t.DependsOn = p.dependencies(value, n)
	default:
		return
	}
	t.FieldLines[key] = n
}

func (p *parser) dependencies(value string, n int) []int {
	value = strings.TrimSuffix(value, sentenceStop)
	if value == dependsNone {
		return nil
	}
	numbers, rest, ok := parseNumbers(value, commitTaskNoun...)
	if !ok || rest != "" {
		p.problem(n, RuleTaskField, "Depends on must be %s", FormDependsOn)
	}
	return numbers
}

func parseNumbers(text string, nouns ...string) ([]int, string, bool) {
	rest, found := text, false
	for _, noun := range nouns {
		if r, ok := strings.CutPrefix(text, noun); ok {
			rest, found = r, true
			break
		}
	}
	first := leadingNumber.FindString(rest)
	if !found || first == "" {
		return nil, text, false
	}
	n, _ := strconv.Atoi(first)
	numbers := []int{n}
	rest = rest[len(first):]
	for {
		m := followingNum.FindStringSubmatch(rest)
		if m == nil {
			return numbers, rest, true
		}
		next, _ := strconv.Atoi(m[2])
		last := numbers[len(numbers)-1]
		if strings.TrimSpace(m[1]) != wordTo {
			numbers = append(numbers, next)
		} else if next < last || next-last > maxRangeSpan {
			return nil, text, false
		} else {
			for k := last + 1; k <= next; k++ {
				numbers = append(numbers, k)
			}
		}
		rest = rest[len(m[0]):]
	}
}

func (p *parser) specLine(line string, n int) {
	value, ok := strings.CutPrefix(line, CriteriaKey)
	if !ok || len(p.doc.Specs) == 0 {
		return
	}
	s := &p.doc.Specs[len(p.doc.Specs)-1]
	s.HasCriteriaLine = true
	numbers, rest, ok := parseNumbers(strings.TrimSpace(value), "")
	if !ok || strings.Trim(rest, ". ") != "" {
		p.problem(n, RuleSpecCriteria, "Criteria must be %s", FormSpecCriteria)
		return
	}
	s.Criteria = numbers
}

func (p *parser) commit(line string, n int) {
	if !numberedLine.MatchString(line) {
		return
	}
	m := commitLine.FindStringSubmatch(line)
	if m == nil {
		p.problem(n, RuleCommitFormat, "commit must be %s", FormCommit)
		return
	}
	tasks, rest, ok := parseNumbers(m[3], commitTaskNoun...)
	if !ok || rest != "" {
		p.problem(n, RuleCommitFormat, "commit tasks must be task n or tasks a, b and c")
		return
	}
	number, _ := strconv.Atoi(m[1])
	p.doc.Commits = append(p.doc.Commits, Commit{Number: number, Subject: m[2], Tasks: tasks, Line: n})
}

func (p *parser) pr(line string, n int) {
	if !numberedLine.MatchString(line) {
		return
	}
	m := prLine.FindStringSubmatch(line)
	if m == nil {
		p.problem(n, RulePRSequence, "PR line must be %s", FormPR)
		return
	}
	number, _ := strconv.Atoi(m[1])
	p.doc.PRs = append(p.doc.PRs, PR{Number: number, Branch: m[2], ThisRun: m[3] != "", Base: m[4], Line: n})
}

func (p *parser) closeFence() {
	f := p.fence
	p.fence = nil
	switch f.info {
	case fenceMermaid:
		p.doc.Diagrams = append(p.doc.Diagrams, newDiagram(f))
	case FenceTree:
		switch p.sub {
		case HeadingCodeTree:
			p.doc.CodeTree = p.tree(f)
		case HeadingTestTree:
			p.doc.TestTree = p.tree(f)
		}
	}
}

func newDiagram(f *fence) Diagram {
	body := withoutFrontMatter(f.body)
	d := Diagram{Body: strings.Join(body, "\n"), Line: f.line}
	for _, line := range body {
		if words := strings.Fields(line); len(words) > 0 && !strings.HasPrefix(words[0], mermaidComment) {
			d.Kind = words[0]
			break
		}
	}
	return d
}

func withoutFrontMatter(body []string) []string {
	start := slices.IndexFunc(body, func(line string) bool { return strings.TrimSpace(line) != "" })
	if start < 0 || strings.TrimSpace(body[start]) != frontMatter {
		return body
	}
	end := slices.IndexFunc(body[start+1:], func(line string) bool { return strings.TrimSpace(line) == frontMatter })
	if end < 0 {
		return body
	}
	blanked := slices.Clone(body)
	clear(blanked[:start+end+2])
	return blanked
}

func splitTreeLine(line string) (depth int, rest string, ok bool) {
	r := []rune(line)
	for len(r) >= TreeLevelWidth {
		switch string(r[:TreeLevelWidth]) {
		case treeBar, treeBlank:
			depth++
			r = r[TreeLevelWidth:]
			continue
		case treeTee, treeLast:
			return depth + 1, string(r[TreeLevelWidth:]), true
		}
		break
	}
	if depth > 0 || len(r) == 0 || strings.ContainsRune(treeIndentRune, r[0]) {
		return 0, "", false
	}
	return 0, string(r), true
}

func validMark(mark string) bool {
	return mark == MarkNew || mark == MarkChanged || mark == MarkDeleted
}

func (p *parser) tree(f *fence) Tree {
	t := Tree{Line: f.line}
	var dirs []string
	rooted := false
	for i, line := range f.body {
		n := f.line + 1 + i
		if strings.TrimSpace(line) == "" {
			continue
		}
		depth, rest, ok := splitTreeLine(line)
		if !ok {
			p.problem(n, RuleTreeFormat, "indent must be %d characters for each level. Example: %q under %q", TreeLevelWidth, strings.Repeat(" ", TreeLevelWidth)+"└── thing/", "└── internal/")
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			p.problem(n, RuleTreeFormat, "the line has no name")
			continue
		}
		name := fields[0]
		isDir := strings.HasSuffix(name, "/")
		if depth == 0 {
			if rooted || !isDir {
				p.problem(n, RuleTreeFormat, "the first line is the only root and ends in /")
			}
			rooted = true
			continue
		}
		if !rooted {
			p.problem(n, RuleTreeFormat, "the root line must come first")
			rooted = true
		}
		mark := ""
		switch {
		case isDir && len(fields) > 1:
			p.problem(n, RuleTreeFormat, "folder %q takes no mark", name)
		case !isDir && (len(fields) != 2 || !validMark(fields[1])):
			p.problem(n, RuleTreeFormat, "file %q needs a mark: %s", name, TreeMarks)
			continue
		case !isDir:
			mark = fields[1]
		}
		if depth-1 > len(dirs) {
			p.problem(n, RuleTreeFormat, "%q skips a level. A child is indented %d more characters than its parent. A bad line above also causes this.", name, TreeLevelWidth)
			continue
		}
		dirs = dirs[:depth-1]
		path := strings.Join(dirs, "") + name
		if isDir {
			dirs = append(dirs, name)
		}
		t.Entries = append(t.Entries, TreeEntry{Path: path, Mark: mark, Line: n, Depth: depth, IsDir: isDir})
	}
	return t
}

const (
	SeamLabel = "Seam:"
	FormSeam  = "`Seam: `pkg.Name`.` in the Test first line"
)

var seamName = regexp.MustCompile(regexp.QuoteMeta(SeamLabel) + " `([^`]+)`")

func seamNames(value string) []string {
	var names []string
	for _, m := range seamName.FindAllStringSubmatch(value, -1) {
		names = append(names, m[1])
	}
	return names
}
