package domain

import (
	"regexp"
	"strconv"
	"strings"
)

const (
	HeadingRuns        = "Runs"
	HeadingMeasurement = "Measurement"
)

const (
	FieldWave     = "Wave"
	FieldBranch   = "Branch"
	FieldDoneWhen = "Done when"
)

const (
	FormRunHeading = "R<n> slug"
	FormRunDepends = "none, R1, R1 and R3 or R1, R2 and R3"
	FormWave       = "a positive number"
	FormBranch     = "type/slug"
)

var (
	ProgramFields = []string{FieldWave, FieldDependsOn, FieldBranch, FieldOwned, FieldDoneWhen}

	runHeading = regexp.MustCompile(`^(R\d+[a-z]?) ([a-z0-9]+(?:-[a-z0-9]+)*)$`)
	runList    = regexp.MustCompile(`,\s*and\s+|,\s*|\s+and\s+`)
	branchForm = regexp.MustCompile(`^[a-z]+/[a-z0-9][a-z0-9._-]*$`)
)

type Program struct {
	Title             string
	Goal              []string
	Runs              []ProgramRun
	MeasurementHeader []string
	MeasurementRows   [][]string
	Decisions         []string
	Lines             []string
	Headings          []Heading
}

type ProgramRun struct {
	ID         string
	Slug       string
	Wave       int
	DependsOn  []string
	Branch     string
	Owned      []string
	DoneWhen   string
	Scope      []string
	Line       int
	FieldLines map[string]int
}

func IsProgram(text string) bool {
	inFence := false
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, trailingSpace)
		switch {
		case inFence:
			inFence = !closesFence(line)
		case opensFence(line):
			inFence = true
		case line == "## "+HeadingRuns:
			return true
		}
	}
	return false
}

type programParser struct {
	prog     Program
	problems []Problem
	section  string
	run      int
}

func ParseProgram(text string) (Program, []Problem) {
	p := &programParser{prog: Program{Lines: strings.Split(text, "\n")}, run: -1}
	inFence := false
	for i, raw := range p.prog.Lines {
		line := strings.TrimRight(raw, trailingSpace)
		switch {
		case inFence:
			inFence = !closesFence(line)
		case opensFence(line):
			inFence = true
		case strings.HasPrefix(line, "### "):
			p.heading(3, strings.TrimPrefix(line, "### "), i+1)
		case strings.HasPrefix(line, "## "):
			p.heading(2, strings.TrimPrefix(line, "## "), i+1)
		case strings.HasPrefix(line, "# ") && p.prog.Title == "":
			p.prog.Title = strings.TrimPrefix(line, "# ")
		default:
			p.content(line, i+1)
		}
	}
	return p.prog, p.problems
}

func (p *programParser) heading(level int, text string, n int) {
	p.prog.Headings = append(p.prog.Headings, Heading{Level: level, Text: text, Line: n})
	p.run = -1
	if level == 2 {
		p.section = text
		return
	}
	if p.section != HeadingRuns {
		return
	}
	m := runHeading.FindStringSubmatch(text)
	if m == nil {
		p.problems = append(p.problems, Problem{Line: n, Rule: RuleRunField, Message: "run heading " + text + " must be ### " + FormRunHeading})
		return
	}
	p.prog.Runs = append(p.prog.Runs, ProgramRun{ID: m[1], Slug: m[2], Line: n, FieldLines: map[string]int{}})
	p.run = len(p.prog.Runs) - 1
}

func (p *programParser) content(line string, n int) {
	switch p.section {
	case HeadingGoal:
		if strings.TrimSpace(line) != "" {
			p.prog.Goal = append(p.prog.Goal, line)
		}
	case HeadingDecisions:
		if item, ok := strings.CutPrefix(line, bulletMark); ok {
			p.prog.Decisions = append(p.prog.Decisions, item)
		}
	case HeadingMeasurement:
		p.measurement(line)
	case HeadingRuns:
		p.runLine(line, n)
	}
}

func (p *programParser) measurement(line string) {
	if !strings.HasPrefix(line, "|") {
		return
	}
	c := cells(line)
	switch {
	case p.prog.MeasurementHeader == nil:
		p.prog.MeasurementHeader = c
	case !isSeparator(c):
		p.prog.MeasurementRows = append(p.prog.MeasurementRows, c)
	}
}

func (p *programParser) runLine(line string, n int) {
	if p.run < 0 {
		return
	}
	r := &p.prog.Runs[p.run]
	if numberedLine.MatchString(line) {
		r.Scope = append(r.Scope, numberedLine.ReplaceAllString(line, ""))
		return
	}
	bullet, ok := strings.CutPrefix(line, bulletMark)
	if !ok {
		return
	}
	key, value, ok := strings.Cut(bullet, ":")
	if !ok {
		return
	}
	value = strings.TrimSpace(value)
	switch key {
	case FieldWave:
		r.Wave, _ = strconv.Atoi(value)
	case FieldDependsOn:
		r.DependsOn = runIDs(value)
	case FieldBranch:
		r.Branch = value
	case FieldOwned:
		r.Owned = append(r.Owned, ownedPaths(value)...)
	case FieldDoneWhen:
		r.DoneWhen = value
	default:
		return
	}
	r.FieldLines[key] = n
}

func runIDs(value string) []string {
	value = strings.TrimSuffix(value, sentenceStop)
	if value == dependsNone || value == "" {
		return nil
	}
	return runList.Split(value, -1)
}

func CheckProgram(p Program) []Problem {
	var problems []Problem
	add := reporter(&problems)
	checkProgramHeadings(p, add)
	checkProgramRuns(p, add)
	checkProgramOverlap(p, add)
	checkSentences(Document{Lines: p.Lines}, add)
	SortProblems(problems)
	return problems
}

func checkProgramHeadings(p Program, add report) {
	doc := Document{Headings: p.Headings}
	for _, text := range []string{HeadingGoal, HeadingRuns} {
		if headingLine(doc, 2, text) == 0 {
			add(1, RuleHeadingMissing, "heading ## %s is missing", text)
		}
	}
	if line := headingLine(doc, 2, HeadingRuns); line != 0 && len(p.Runs) == 0 {
		add(line, RuleHeadingMissing, "## %s has no run", HeadingRuns)
	}
}

func checkProgramRuns(p Program, add report) {
	byID := make(map[string]ProgramRun, len(p.Runs))
	for _, r := range p.Runs {
		if _, ok := byID[r.ID]; ok {
			add(r.Line, RuleRunField, "run id %s is used twice", r.ID)
			continue
		}
		byID[r.ID] = r
	}
	for _, r := range p.Runs {
		checkRunFields(r, add)
		for _, d := range r.DependsOn {
			line := r.FieldLines[FieldDependsOn]
			dep, ok := byID[d]
			switch {
			case !ok:
				add(line, RuleRunDepends, "run %s depends on %s, which is not a run of the plan", r.ID, d)
			case dep.Wave >= r.Wave:
				add(line, RuleRunDepends, "run %s in wave %d depends on %s in wave %d, which is not an earlier wave", r.ID, r.Wave, d, dep.Wave)
			}
		}
	}
}

func checkRunFields(r ProgramRun, add report) {
	for _, f := range ProgramFields {
		if _, ok := r.FieldLines[f]; !ok {
			add(r.Line, RuleRunField, "run %s has no %s: line", r.ID, f)
		}
	}
	has := func(f string) bool {
		_, ok := r.FieldLines[f]
		return ok
	}
	if has(FieldWave) && r.Wave < 1 {
		add(r.FieldLines[FieldWave], RuleRunField, "%s must be %s", FieldWave, FormWave)
	}
	if has(FieldBranch) && !branchForm.MatchString(r.Branch) {
		add(r.FieldLines[FieldBranch], RuleRunField, "%s must be %s", FieldBranch, FormBranch)
	}
	if has(FieldOwned) && len(r.Owned) == 0 {
		add(r.FieldLines[FieldOwned], RuleRunField, "%s needs a path in backticks", FieldOwned)
	}
	if has(FieldDoneWhen) && r.DoneWhen == "" {
		add(r.FieldLines[FieldDoneWhen], RuleRunField, "%s needs text", FieldDoneWhen)
	}
	if len(r.Scope) == 0 {
		add(r.Line, RuleRunField, "run %s has no scope step", r.ID)
	}
}

func checkProgramOverlap(p Program, add report) {
	for i, b := range p.Runs {
		for _, a := range p.Runs[:i] {
			if a.Wave != b.Wave {
				continue
			}
			if shared, ok := sharedPath(a.Owned, b.Owned); ok {
				add(lineOr(b.FieldLines[FieldOwned], b.Line), RuleRunOverlap, "runs %s and %s in wave %d both own %s", a.ID, b.ID, a.Wave, shared)
			}
		}
	}
}

func sharedPath(a, b []string) (string, bool) {
	for _, path := range a {
		if owns(b, path) {
			return path, true
		}
	}
	for _, path := range b {
		if owns(a, path) {
			return path, true
		}
	}
	return "", false
}
