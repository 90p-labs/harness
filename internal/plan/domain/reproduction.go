package domain

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const (
	noneKey         = "none:"
	minSymptomChars = 8
)

var (
	hypothesisLine = regexp.MustCompile(`^(\d+)\. If .+, then .+\. Result: ([A-Za-z]+)\.$`)
	symptomText    = regexp.MustCompile("^`([^`]+)`$")
	genericSymptom = []string{"exit status", "--- fail", "build failed", "fail:", "fail ", "fail\t"}
	resultWords    = []string{ResultConfirmed, ResultRefuted, ResultUntested}
)

const (
	ResultConfirmed = "confirmed"
	ResultRefuted   = "refuted"
	ResultUntested  = "untested"
)

// reproductionHeading keeps the first heading line and the line of the second one, which the rule reports.
func (p *parser) reproductionHeading(n int) {
	r := &p.doc.Reproduction
	switch {
	case r.Line == 0:
		r.Line = n
	case r.SecondHeading == 0:
		r.SecondHeading = n
	}
}

func (p *parser) reproductionLine(line string, n int) {
	r := &p.doc.Reproduction
	if m := hypothesisLine.FindStringSubmatch(line); m != nil {
		number, _ := strconv.Atoi(m[1])
		r.Hypotheses = append(r.Hypotheses, Hypothesis{Number: number, Result: strings.ToLower(m[2]), Formed: true, Line: n})
		return
	}
	if numberedLine.MatchString(line) {
		number, _ := strconv.Atoi(leadingNumber.FindString(line))
		r.Hypotheses = append(r.Hypotheses, Hypothesis{Number: number, Line: n})
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
	entry := ReproLine{Value: strings.TrimSpace(value), Line: n}
	switch key {
	case ReproCriterion:
		r.Criterion = entry
	case ReproSymptom:
		r.Symptom = entry
	case ReproHumanOnly:
		r.HumanOnly = entry
	case ReproRegression:
		r.Regression = entry
	}
}

func checkReproduction(doc Document, add report) {
	r := doc.Reproduction
	checkReproductionCause(doc, add)
	if doc.Cause != CauseBug || r.Line == 0 {
		return
	}
	checkReproductionProof(doc, add)
	checkHypotheses(r, add)
	checkRegressionTest(doc, add)
}

func checkReproductionCause(doc Document, add report) {
	r := doc.Reproduction
	bug := doc.Cause == CauseBug
	fix := slices.IndexFunc(doc.Commits, func(c Commit) bool { return c.Type() == CommitTypeFix })
	if r.SecondHeading > 0 {
		add(r.SecondHeading, RuleReproduction, "a second ## %s heading: keep one section", HeadingReproduction)
	}
	switch {
	case doc.CauseLine > 0 && !bug:
		add(doc.CauseLine, RuleReproduction, "%s must be `%s %s` or absent", CausePrefix, CausePrefix, CauseBug)
	case bug && r.Line == 0:
		add(doc.CauseLine, RuleReproduction, "a bug plan needs ## %s", HeadingReproduction)
	case !bug && r.Line > 0:
		add(r.Line, RuleReproduction, "## %s needs the line `%s %s` under ## %s", HeadingReproduction, CausePrefix, CauseBug, HeadingGoal)
	case !bug && fix >= 0:
		add(doc.Commits[fix].Line, RuleReproduction, "a plan with a fix commit needs the line `%s %s` and ## %s", CausePrefix, CauseBug, HeadingReproduction)
	}
}

// ReproductionRecord is the state and output tail of one criterion proof for the current files.
type ReproductionRecord struct {
	Passed bool
	Tail   string
}

// ReproductionReader gives the record of a criterion proof. The bool is false when no record exists for the current files.
type ReproductionReader interface {
	Reproduction(ctx context.Context, criterion int) (ReproductionRecord, bool, error)
}

// VerifyReproduction gives the problems of the rule reproduction-red for a bug plan with a criterion.
// It skips a plan with no bug cause, a plan with Human only and a nil reader.
func VerifyReproduction(ctx context.Context, doc Document, reader ReproductionReader) ([]Problem, error) {
	r := doc.Reproduction
	if reader == nil || doc.Cause != CauseBug || r.Line == 0 || r.HumanOnly.Present() || !r.Criterion.Present() {
		return nil, nil
	}
	number, err := strconv.Atoi(r.Criterion.Value)
	if err != nil {
		return nil, nil
	}
	symptom := ""
	if m := symptomText.FindStringSubmatch(r.Symptom.Value); m != nil {
		symptom = m[1]
	}
	problem := func(format string, args ...any) []Problem {
		return []Problem{{Line: r.Criterion.Line, Rule: RuleReproductionRed, Message: fmt.Sprintf(format, args...)}}
	}
	if symptom == "" {
		return problem("criterion %d has no %s text in backticks, so no failure can match it", number, ReproSymptom), nil
	}
	record, found, err := reader.Reproduction(ctx, number)
	if err != nil {
		return nil, err
	}
	switch {
	case !found:
		return problem("criterion %d has no record for the current files: run `harness verify --criterion %d` and read its output, because a setup failure or a gate veto saves no record", number, number), nil
	case record.Passed:
		return problem("the proof of criterion %d passed, so it does not reproduce the bug", number), nil
	case !strings.Contains(record.Tail, symptom):
		return problem("the proof of criterion %d failed, and its output does not hold the %s text %q: fix the proof or the symptom", number, ReproSymptom, symptom), nil
	}
	return nil, nil
}

func checkReproductionProof(doc Document, add report) {
	r := doc.Reproduction
	switch {
	case r.Criterion.Present() == r.HumanOnly.Present():
		add(r.Line, RuleReproduction, "give one of `- %s: n` and `- %s: reason`", ReproCriterion, ReproHumanOnly)
	case r.HumanOnly.Present():
		if r.Symptom.Present() {
			add(r.Symptom.Line, RuleReproduction, "%s stands with %s: delete it", ReproSymptom, ReproHumanOnly)
		}
		if r.HumanOnly.Value == "" {
			add(r.HumanOnly.Line, RuleReproduction, "%s needs a reason", ReproHumanOnly)
		}
	default:
		checkCriterionSymptom(doc, add)
	}
}

func checkCriterionSymptom(doc Document, add report) {
	r := doc.Reproduction
	number, err := strconv.Atoi(r.Criterion.Value)
	idx := slices.IndexFunc(doc.Criteria, func(c Criterion) bool { return c.Number == number })
	if err != nil || idx < 0 {
		add(r.Criterion.Line, RuleReproduction, "%s %q names no criterion of the table", ReproCriterion, r.Criterion.Value)
	}
	if !r.Symptom.Present() {
		add(r.Criterion.Line, RuleReproduction, "%s needs a `- %s: `text`` line", ReproCriterion, ReproSymptom)
		return
	}
	m := symptomText.FindStringSubmatch(r.Symptom.Value)
	if m == nil {
		add(r.Symptom.Line, RuleReproduction, "%s must be one text in backticks", ReproSymptom)
		return
	}
	text := m[1]
	lower := strings.ToLower(text)
	switch {
	case len([]rune(text)) < minSymptomChars:
		add(r.Symptom.Line, RuleReproduction, "%s has fewer than %d characters", ReproSymptom, minSymptomChars)
	case idx >= 0 && strings.Contains(doc.Criteria[idx].Proof, text):
		add(r.Symptom.Line, RuleReproduction, "%s is a part of the proof command", ReproSymptom)
	case slices.ContainsFunc(genericSymptom, func(g string) bool { return strings.HasPrefix(lower, g) }):
		add(r.Symptom.Line, RuleReproduction, "%s is generic: quote the text that shows the reported reason", ReproSymptom)
	}
}

func checkHypotheses(r Reproduction, add report) {
	if n := len(r.Hypotheses); n < MinHypotheses || n > MaxHypotheses {
		add(r.Line, RuleReproduction, "the section needs %d to %d hypotheses, has %d", MinHypotheses, MaxHypotheses, n)
	}
	firstConfirmed, firstUntested := 0, 0
	for i, h := range r.Hypotheses {
		switch {
		case !h.Formed:
			add(h.Line, RuleReproduction, "hypothesis must be %s", FormHypothesis)
			continue
		case h.Number != i+1:
			add(h.Line, RuleReproduction, "hypothesis number is %d, want %d", h.Number, i+1)
		case !slices.Contains(resultWords, h.Result):
			add(h.Line, RuleReproduction, "result must be %s", strings.Join(resultWords, ", "))
			continue
		}
		if h.Result == ResultConfirmed && firstConfirmed == 0 {
			firstConfirmed = h.Line
		}
		if h.Result == ResultUntested && firstUntested == 0 {
			firstUntested = h.Line
		}
	}
	if r.HumanOnly.Present() {
		return
	}
	switch {
	case firstConfirmed == 0:
		add(r.Line, RuleReproduction, "no hypothesis is %s", ResultConfirmed)
	case firstUntested != 0 && firstUntested < firstConfirmed:
		add(firstUntested, RuleReproduction, "a %s hypothesis ranks above the first %s one", ResultUntested, ResultConfirmed)
	}
}

func checkRegressionTest(doc Document, add report) {
	r := doc.Reproduction.Regression
	if !r.Present() {
		add(doc.Reproduction.Line, RuleReproduction, "the section needs `- %s: `TestName`` or `- %s: none: reason`", ReproRegression, ReproRegression)
		return
	}
	if reason, ok := doc.Reproduction.NoSeamReason(); ok {
		if reason == "" {
			add(r.Line, RuleReproduction, "%s none: needs a reason", ReproRegression)
		}
		return
	}
	m := symptomText.FindStringSubmatch(r.Value)
	if m == nil {
		add(r.Line, RuleReproduction, "%s must be one name in backticks or none: reason", ReproRegression)
		return
	}
	named := slices.ContainsFunc(doc.Criteria, func(c Criterion) bool { return strings.Contains(c.Proof, m[1]) }) ||
		slices.ContainsFunc(doc.Tasks, func(t Task) bool { return strings.Contains(t.TestFirst, m[1]) })
	if !named {
		add(r.Line, RuleReproduction, "%s %s is in no proof and no %s line", ReproRegression, m[1], FieldTestFirst)
	}
}

const (
	CausePrefix = "Cause:"
	CauseBug    = "bug"
)

const (
	ReproCriterion  = "Criterion"
	ReproSymptom    = "Symptom"
	ReproHumanOnly  = "Human only"
	ReproRegression = "Regression test"

	FormHypothesis = "n. If cause, then prediction. Result: word."

	MinHypotheses = 3
	MaxHypotheses = 5
)

// Reproduction is the parsed section `## Reproduction`. A zero Line means the plan has no such section.
type Reproduction struct {
	Line          int
	SecondHeading int // the line of a second heading of the section, or 0
	Criterion     ReproLine
	Symptom       ReproLine
	HumanOnly     ReproLine
	Regression    ReproLine
	Hypotheses    []Hypothesis
}

// NoSeamReason gives the reason of `- Regression test: none: <reason>`. The bool is false for any other value.
func (r Reproduction) NoSeamReason() (string, bool) {
	reason, ok := strings.CutPrefix(r.Regression.Value, noneKey)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(reason), true
}

// ReproLine is the value of one bullet of the section. A zero Line means that the bullet is absent.
type ReproLine struct {
	Value string
	Line  int
}

func (l ReproLine) Present() bool { return l.Line > 0 }

// Hypothesis is one numbered line of the section. Formed is false when the line has not the form of FormHypothesis.
type Hypothesis struct {
	Number int
	Result string
	Formed bool
	Line   int
}
