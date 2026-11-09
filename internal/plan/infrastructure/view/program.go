package view

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"slices"
	"strings"

	"github.com/90p-labs/harness/internal/plan/domain"
)

//go:embed program.html.tmpl
var programSource string

const (
	keyRun            = "run-"
	anchorWaves       = "waves"
	anchorDependency  = "dependencies"
	anchorMeasurement = "measurement"
	anchorDecisions   = "decisions"
	groupProgram      = "Program"
	flowchartHead     = "flowchart LR"
	nodeFormat        = "    %s[\"%s %s\"]\n"
	edgeFormat        = "    %s --> %s\n"
)

var programTemplate = template.Must(template.New("program").Parse(programSource))

type programPage struct {
	programModel
	assets
}

type programCounts struct{ Runs, Waves, Problems int }

type runCard struct {
	Key, ID, Slug, Branch string
	DoneWhen              template.HTML
	Owned                 []string
	Scope                 []template.HTML
	After                 []chip
}

type programWave struct {
	Number int
	Runs   []runCard
}

type programModel struct {
	Title     string
	Goal      []template.HTML
	Counts    programCounts
	GroupName string
	GroupQA   string
	Nav       []navGroup
	Waves     []programWave
	Header    []template.HTML
	Rows      [][]template.HTML
	Decisions []template.HTML
	Problems  []problemRow
	Source    []sourceLine
	Data      planData
}

func renderProgram(in domain.ViewInput, a assets) ([]byte, error) {
	var buf bytes.Buffer
	if err := programTemplate.Execute(&buf, programPage{programModel: newProgramModel(in), assets: a}); err != nil {
		return nil, fmt.Errorf("render program page: %w", err)
	}
	return buf.Bytes(), nil
}

func newProgramModel(in domain.ViewInput) programModel {
	prog := *in.Program
	m := programModel{
		Title:     strings.TrimPrefix(prog.Title, planPrefix),
		GroupName: groupProgram,
		GroupQA:   GroupQA,
		Waves:     programWaves(prog),
		Problems:  problemRows(in.Problems, len(prog.Lines)),
		Source:    sourceLines(prog.Lines, in.Problems),
		Data:      planData{Diagrams: []string{dependencyChart(prog.Runs)}},
	}
	for _, line := range prog.Goal {
		m.Goal = append(m.Goal, inline(line))
	}
	for _, cell := range prog.MeasurementHeader {
		m.Header = append(m.Header, inline(cell))
	}
	for _, row := range prog.MeasurementRows {
		cells := make([]template.HTML, len(row))
		for i, cell := range row {
			cells[i] = inline(cell)
		}
		m.Rows = append(m.Rows, cells)
	}
	for _, d := range prog.Decisions {
		m.Decisions = append(m.Decisions, inline(d))
	}
	m.Counts = programCounts{Runs: len(prog.Runs), Waves: len(m.Waves), Problems: len(in.Problems)}
	m.Nav = programNav(m)
	return m
}

func programNav(m programModel) []navGroup {
	items := []navItem{
		{Anchor: anchorSummary, Label: "Goal"},
		{Anchor: anchorWaves, Label: "Runs by wave", Count: itoa(m.Counts.Runs)},
		{Anchor: anchorDependency, Label: "Dependencies"},
	}
	if len(m.Rows) > 0 {
		items = append(items, navItem{Anchor: anchorMeasurement, Label: "Measurement"})
	}
	if len(m.Decisions) > 0 {
		items = append(items, navItem{Anchor: anchorDecisions, Label: "Decisions"})
	}
	return []navGroup{
		{"01", groupProgram, items},
		{"02", GroupQA, []navItem{
			{Anchor: anchorProblems, Label: "Problems", Count: itoa(m.Counts.Problems), Bad: m.Counts.Problems > 0},
			{Anchor: anchorSource, Label: "Source", Count: itoa(len(m.Source))},
		}},
	}
}

func programWaves(prog domain.Program) []programWave {
	var numbers []int
	for _, r := range prog.Runs {
		if !slices.Contains(numbers, r.Wave) {
			numbers = append(numbers, r.Wave)
		}
	}
	slices.Sort(numbers)
	known := map[string]bool{}
	for _, r := range prog.Runs {
		known[r.ID] = true
	}
	waves := make([]programWave, len(numbers))
	for i, n := range numbers {
		waves[i].Number = n
		for _, r := range prog.Runs {
			if r.Wave == n {
				waves[i].Runs = append(waves[i].Runs, runCardOf(r, known))
			}
		}
	}
	return waves
}

func runCardOf(r domain.ProgramRun, known map[string]bool) runCard {
	card := runCard{
		Key: keyRun + r.ID, ID: r.ID, Slug: r.Slug, Branch: r.Branch,
		DoneWhen: inline(r.DoneWhen), Owned: r.Owned,
	}
	for _, step := range r.Scope {
		card.Scope = append(card.Scope, inline(step))
	}
	for _, d := range r.DependsOn {
		c := chip{Label: d}
		if known[d] {
			c.Href = keyRun + d
		}
		card.After = append(card.After, c)
	}
	return card
}

func dependencyChart(runs []domain.ProgramRun) string {
	var b strings.Builder
	b.WriteString(flowchartHead + "\n")
	for _, r := range runs {
		fmt.Fprintf(&b, nodeFormat, r.ID, r.ID, r.Slug)
	}
	for _, r := range runs {
		for _, d := range r.DependsOn {
			fmt.Fprintf(&b, edgeFormat, d, r.ID)
		}
	}
	return b.String()
}
