package view

import (
	"bytes"
	"embed"
	"encoding/base64"
	"fmt"
	"html"
	"html/template"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/jorge07/harness/internal/plan/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

//go:embed page.html.tmpl
var pageSource string

//go:embed view.css
var pageCSS string

//go:embed view.js
var pageJS string

//go:embed assets/mermaid.min.js
var mermaidJS string

//go:embed assets/*.woff2
var fontFiles embed.FS

const (
	GroupProduct = "Product"
	GroupSpecs   = "Tech specs"
	GroupTasks   = "Tasks"
	GroupQA      = "QA and verification"
	GroupSignals = "Signals"
)

const (
	keyCriterion = "c"
	keyTask      = "t"
	keySpec      = "s-"
	keyFile      = "f-"
	keyCommit    = "k"
	keySignal    = "g"
	keyPR        = "pr"
	keyLine      = "L"
)

const (
	anchorSummary  = "summary"
	anchorCriteria = "criteria"
	anchorNotes    = "notes"
	anchorSpecs    = "specs"
	anchorTrees    = "trees"
	anchorTasks    = "tasks"
	anchorDelivery = "delivery"
	anchorProblems = "problems"
	anchorMatrix   = "trace-matrix"
	anchorProofs   = "proofs"
	anchorSource   = "source"
	anchorSignals  = "signals"
)

const (
	headingRisks     = "Risks"
	headingNonGoals  = "Non-goals"
	headingEdgeCases = "Edge cases"
)

const (
	fenceMark      = "```"
	criteriaPrefix = "Criteria:"
	trackPrefix    = "Track:"
	bulletPrefix   = "- "
	titlePrefix    = "# "
	planPrefix     = "Plan: "
	sketchSVG      = "<svg"
	sentenceBreak  = ". "
	fontFaceFormat = "@font-face{font-family:'%s';src:url(data:font/woff2;base64,%s) format('woff2');font-weight:%s;font-display:swap}"
	tokensFormat   = ":root{--added:%s;--modified:%s;--deleted:%s}"
)

var (
	pageTemplate = template.Must(template.New("page").Funcs(template.FuncMap{"cls": stateClass, "dict": dict}).Parse(pageSource))
	codeSpan     = regexp.MustCompile("`([^`]+)`")
	sketchLink   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	markColor    = regexp.MustCompile(`color:(#[0-9a-fA-F]{6})`)
	nonAnchor    = regexp.MustCompile(`[^A-Za-z0-9]+`)
)

const headingLevel = 2

var fontFaces = []struct{ file, family, weight string }{
	{"InstrumentSerif.woff2", "Instrument Serif", "400"},
	{"SourceSans3.woff2", "Source Sans 3", "200 900"},
	{"IBMPlexMono.woff2", "IBM Plex Mono", "400"},
	{"Excalifont.woff2", "Excalifont", "400"},
}

type Renderer struct{}

func NewRenderer() Renderer {
	return Renderer{}
}

type page struct {
	model
	Fonts   template.CSS
	Tokens  template.CSS
	CSS     template.CSS
	JS      template.JS
	Mermaid template.JS
}

func (Renderer) Render(in domain.ViewInput) ([]byte, error) {
	fonts, err := fontCSS()
	if err != nil {
		return nil, err
	}
	p := page{
		model:   newBuilder(in).model(),
		Fonts:   fonts,
		Tokens:  tokenCSS(),
		CSS:     template.CSS(pageCSS),
		JS:      template.JS(pageJS),
		Mermaid: template.JS(mermaidJS),
	}
	var buf bytes.Buffer
	if err := pageTemplate.Execute(&buf, p); err != nil {
		return nil, fmt.Errorf("render plan page: %w", err)
	}
	return buf.Bytes(), nil
}

func fontCSS() (template.CSS, error) {
	var css strings.Builder
	for _, f := range fontFaces {
		raw, err := fontFiles.ReadFile("assets/" + f.file)
		if err != nil {
			return "", fmt.Errorf("read font %s: %w", f.file, err)
		}
		fmt.Fprintf(&css, fontFaceFormat, f.family, base64.StdEncoding.EncodeToString(raw), f.weight)
	}
	return template.CSS(css.String()), nil
}

func tokenCSS() template.CSS {
	color := func(mark shareddomain.Mark) string {
		return markColor.FindStringSubmatch(shareddomain.MarkStyles[mark])[1]
	}
	return template.CSS(fmt.Sprintf(tokensFormat,
		color(shareddomain.MarkNew), color(shareddomain.MarkChanged), color(shareddomain.MarkRemoved)))
}

func stateClass(state string) string {
	return strings.ReplaceAll(state, " ", "-")
}

func dict(pairs ...any) map[string]any {
	out := make(map[string]any, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out[pairs[i].(string)] = pairs[i+1]
	}
	return out
}

func inline(text string) template.HTML {
	return template.HTML(codeSpan.ReplaceAllString(html.EscapeString(text), "<code>$1</code>"))
}

type chip struct{ Href, Label, Title string }

type navItem struct {
	Anchor, Label, Count string
	Bad                  bool
}

type navGroup struct {
	Number, Name string
	Items        []navItem
}

type groupNames struct{ Product, Specs, Tasks, QA, Signals string }

type counts struct{ Criteria, Tasks, Files, Problems int }

type problemRow struct {
	Anchor, Rule, Message string
	Line                  int
}

type criterionRow struct {
	Key, Proof, State             string
	Number                        int
	Text                          template.HTML
	Tasks, Specs, Commits, Signal []chip
}

type matrixRow struct {
	Key, Text, State      string
	Number                int
	Dots                  []bool
	Specs, Files, Signals int
}

type matrixView struct {
	Tasks []chip
	Rows  []matrixRow
}

type diagramRow struct {
	Index        int
	Kind, Source string
}

type sketchRow struct {
	Title, Path, Error string
	SVG                template.HTML
}

type specRow struct {
	Key, Title string
	Prose      []template.HTML
	Criteria   []chip
	Diagrams   []diagramRow
	Sketches   []sketchRow
}

type treeRow struct {
	ID, Name, Mark, State string
	Depth                 int
	Dir                   bool
	Owners                []chip
}

type treeView struct {
	Title string
	Rows  []treeRow
}

type taskRow struct {
	Key, Name, Side     string
	Number              int
	TestFirst           template.HTML
	Owned               []string
	DoD, After, Commits []chip
}

type waveRow struct {
	Number int
	Tasks  []taskRow
}

type commitRow struct {
	Key, Subject, State string
	Number              int
	Tasks               []chip
}

type prRow struct {
	Key, Branch, Base string
	ThisRun           bool
}

type signalRow struct {
	Key, Signal, Source, Query, Breach, Window, Panel string
	Criterion                                         []chip
}

type noteBlock struct {
	Title string
	Items []template.HTML
}

type sourceLine struct {
	N    int
	Text string
	Bad  bool
}

type planData struct {
	Graph    map[string][]string `json:"graph"`
	Labels   map[string]string   `json:"labels"`
	State    map[string]string   `json:"state"`
	Diagrams []string            `json:"diagrams"`
}

type model struct {
	Title      string
	Track      string
	Run        domain.ViewRun
	Goal       []template.HTML
	Counts     counts
	G          groupNames
	Nav        []navGroup
	Problems   []problemRow
	Criteria   []criterionRow
	Notes      []noteBlock
	Specs      []specRow
	Trees      []treeView
	Waves      []waveRow
	Commits    []commitRow
	PRs        []prRow
	Matrix     matrixView
	Source     []sourceLine
	Signals    []signalRow
	Regression []template.HTML
	Data       planData
}

type builder struct {
	in         domain.ViewInput
	states     domain.TraceStates
	specKeys   map[string]string
	fileKeys   map[string]string
	signalKeys map[int]string
	anchored   map[string]bool
	diagrams   []string
}

func newBuilder(in domain.ViewInput) *builder {
	b := &builder{
		in:         in,
		states:     domain.TraceState(in.Doc, in.Branch, in.Proofs),
		specKeys:   map[string]string{},
		fileKeys:   map[string]string{},
		signalKeys: map[int]string{},
		anchored:   map[string]bool{},
	}
	for i, s := range in.Doc.Specs {
		b.specKeys[s.Title] = number(keySpec, i+1)
	}
	for i, s := range in.Doc.Signals {
		b.signalKeys[s.Line] = number(keySignal, i+1)
	}
	taken := map[string]bool{}
	for _, tree := range b.trees() {
		for _, e := range tree.Entries {
			if _, seen := b.fileKeys[e.Path]; e.IsDir || seen {
				continue
			}
			key := keyFile + slug(e.Path)
			for n := 2; taken[key]; n++ {
				key = keyFile + slug(e.Path) + "-" + itoa(n)
			}
			taken[key] = true
			b.fileKeys[e.Path] = key
		}
	}
	return b
}

func (b *builder) trees() []domain.Tree {
	return []domain.Tree{b.in.Doc.CodeTree, b.in.Doc.TestTree}
}

func (b *builder) model() model {
	doc := b.in.Doc
	m := model{
		Title:    title(doc),
		Track:    b.in.Run.Track,
		Run:      b.in.Run,
		G:        groupNames{GroupProduct, GroupSpecs, GroupTasks, GroupQA, GroupSignals},
		Problems: b.problems(),
		Criteria: b.criteria(),
		Notes:    b.notes(),
		Specs:    b.specs(),
		Trees:    b.treeViews(),
		Waves:    b.waves(),
		Commits:  b.commits(),
		PRs:      b.prs(),
		Matrix:   b.matrix(),
		Source:   b.source(),
		Signals:  b.signals(),
	}
	if m.Track == "" {
		m.Track = doc.Track
	}
	for _, h := range proseOf(doc, domain.HeadingGoal) {
		m.Goal = append(m.Goal, inline(h))
	}
	for _, h := range proseOf(doc, domain.HeadingRegression) {
		m.Regression = append(m.Regression, inline(h))
	}
	m.Counts = counts{Criteria: len(doc.Criteria), Tasks: len(doc.Tasks), Files: len(b.fileKeys), Problems: len(b.in.Problems)}
	m.Nav = b.nav(m)
	m.Data = b.data()
	return m
}

func (b *builder) nav(m model) []navGroup {
	doc := b.in.Doc
	product := []navItem{{Anchor: anchorSummary, Label: "Goal"}, {Anchor: anchorCriteria, Label: "Criteria", Count: itoa(len(doc.Criteria))}}
	if len(m.Notes) > 0 {
		product = append(product, navItem{Anchor: anchorNotes, Label: "Decisions and risks"})
	}
	return []navGroup{
		{"01", GroupProduct, product},
		{"02", GroupSpecs, []navItem{
			{Anchor: anchorSpecs, Label: "Diagrams and sketches", Count: itoa(len(doc.Specs))},
			{Anchor: anchorTrees, Label: "Files", Count: itoa(len(b.fileKeys))},
		}},
		{"03", GroupTasks, []navItem{
			{Anchor: anchorTasks, Label: "Tasks by wave", Count: itoa(len(doc.Tasks))},
			{Anchor: anchorDelivery, Label: "Commits and PR", Count: itoa(len(doc.Commits))},
		}},
		{"04", GroupQA, []navItem{
			{Anchor: anchorProblems, Label: "Problems", Count: itoa(len(b.in.Problems)), Bad: len(b.in.Problems) > 0},
			{Anchor: anchorMatrix, Label: "Trace and proofs"},
			{Anchor: anchorProofs, Label: "Proof commands"},
			{Anchor: anchorSource, Label: "Source", Count: itoa(len(doc.Lines))},
		}},
		{"05", GroupSignals, []navItem{{Anchor: anchorSignals, Label: "Post-deploy signals", Count: itoa(len(doc.Signals))}}},
	}
}

func (b *builder) problems() []problemRow {
	rows := make([]problemRow, len(b.in.Problems))
	for i, p := range b.in.Problems {
		rows[i] = problemRow{Rule: string(p.Rule), Message: p.Message, Line: p.Line}
		if p.Line >= 1 && p.Line <= len(b.in.Doc.Lines) {
			rows[i].Anchor = number(keyLine, p.Line)
		}
	}
	return rows
}

func (b *builder) criterionChip(n int) chip {
	for _, c := range b.in.Doc.Criteria {
		if c.Number == n {
			return chip{Href: number(keyCriterion, n), Label: itoa(n)}
		}
	}
	return chip{Label: itoa(n)}
}

func (b *builder) taskChip(n int) chip {
	for _, t := range b.in.Doc.Tasks {
		if t.Number == n {
			return chip{Href: number(keyTask, n), Label: "T" + itoa(n), Title: t.Name}
		}
	}
	return chip{Label: "T" + itoa(n)}
}

func (b *builder) taskChips(numbers []int) []chip {
	chips := make([]chip, len(numbers))
	for i, n := range numbers {
		chips[i] = b.taskChip(n)
	}
	return chips
}

func (b *builder) commitChips(numbers []int) []chip {
	chips := make([]chip, len(numbers))
	for i, n := range numbers {
		chips[i] = chip{Href: number(keyCommit, n), Label: "#" + itoa(n)}
	}
	return chips
}

func (b *builder) criteria() []criterionRow {
	rows := make([]criterionRow, len(b.in.Doc.Criteria))
	for i, c := range b.in.Doc.Criteria {
		links := b.in.Links.Criteria[c.Number]
		rows[i] = criterionRow{
			Key: number(keyCriterion, c.Number), Number: c.Number, Text: inline(c.Text),
			Proof: strings.Trim(c.Proof, "`"), State: b.states.Proofs[c.Number],
			Tasks: b.taskChips(links.Tasks), Commits: b.commitChips(links.Commits),
		}
		for _, title := range links.Specs {
			rows[i].Specs = append(rows[i].Specs, chip{Href: b.specKeys[title], Label: title})
		}
		if links.Signal != nil {
			rows[i].Signal = []chip{{Href: b.signalKeys[links.Signal.Line], Label: links.Signal.Signal}}
		}
	}
	return rows
}

func (b *builder) notes() []noteBlock {
	var blocks []noteBlock
	for _, name := range []string{domain.HeadingDecisions, headingRisks, headingNonGoals, headingEdgeCases} {
		block := noteBlock{Title: name}
		for _, item := range proseOf(b.in.Doc, name) {
			block.Items = append(block.Items, inline(item))
		}
		if len(block.Items) > 0 {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func (b *builder) specs() []specRow {
	doc := b.in.Doc
	rows := make([]specRow, len(doc.Specs))
	for i, s := range doc.Specs {
		end := sectionEnd(doc, s.Line)
		rows[i] = specRow{Key: number(keySpec, i+1), Title: s.Title}
		for _, line := range proseBetween(doc, s.Line+1, end) {
			rows[i].Prose = append(rows[i].Prose, inline(line))
		}
		for _, n := range s.Criteria {
			rows[i].Criteria = append(rows[i].Criteria, b.criterionChip(n))
		}
		for _, d := range doc.Diagrams {
			if d.Line > s.Line && d.Line < end {
				rows[i].Diagrams = append(rows[i].Diagrams, b.diagram(d))
			}
		}
		for _, sk := range doc.Sketches {
			if sk.Line > s.Line && sk.Line < end {
				rows[i].Sketches = append(rows[i].Sketches, b.sketch(sk))
			}
		}
	}
	return rows
}

func (b *builder) diagram(d domain.Diagram) diagramRow {
	source, ok := b.in.Charts[d.Line]
	if !ok {
		source = d.Body
	}
	b.diagrams = append(b.diagrams, source)
	return diagramRow{Index: len(b.diagrams) - 1, Kind: d.Kind, Source: d.Body}
}

func (b *builder) sketch(s domain.Sketch) sketchRow {
	row := sketchRow{Title: s.Title, Path: s.Path}
	out := b.in.Sketches[s.Path]
	if strings.HasPrefix(string(out), sketchSVG) {
		row.SVG = template.HTML(out)
	} else {
		row.Error = string(out)
	}
	return row
}

func (b *builder) treeViews() []treeView {
	titles := []string{domain.HeadingCodeTree, domain.HeadingTestTree}
	views := make([]treeView, 0, len(titles))
	for i, tree := range b.trees() {
		view := treeView{Title: titles[i]}
		for _, e := range tree.Entries {
			view.Rows = append(view.Rows, b.treeRow(e))
		}
		views = append(views, view)
	}
	return views
}

func (b *builder) treeRow(e domain.TreeEntry) treeRow {
	trimmed := strings.TrimSuffix(e.Path, "/")
	name := trimmed[strings.LastIndex(trimmed, "/")+1:]
	if e.IsDir {
		return treeRow{Name: name + "/", Depth: e.Depth, Dir: true}
	}
	row := treeRow{Name: name, Mark: e.Mark, Depth: e.Depth, State: b.states.Files[e.Path]}
	if key := b.fileKeys[e.Path]; !b.anchored[key] {
		row.ID = key
		b.anchored[key] = true
	}
	for _, t := range b.in.Doc.Tasks {
		if slices.Contains(b.in.Links.Tasks[t.Number].Files, e.Path) {
			row.Owners = append(row.Owners, b.taskChip(t.Number))
		}
	}
	return row
}

func (b *builder) waves() []waveRow {
	doc := b.in.Doc
	numbers, err := domain.Waves(doc.Tasks)
	if err != nil {
		numbers = [][]int{nil}
		for _, t := range doc.Tasks {
			numbers[0] = append(numbers[0], t.Number)
		}
	}
	byNumber := make(map[int]domain.Task, len(doc.Tasks))
	for _, t := range doc.Tasks {
		byNumber[t.Number] = t
	}
	rows := make([]waveRow, len(numbers))
	for i, wave := range numbers {
		rows[i].Number = i + 1
		for _, n := range wave {
			rows[i].Tasks = append(rows[i].Tasks, b.task(byNumber[n]))
		}
	}
	return rows
}

func (b *builder) task(t domain.Task) taskRow {
	row := taskRow{
		Key: number(keyTask, t.Number), Number: t.Number, Name: t.Name, Side: t.Side,
		TestFirst: inline(t.TestFirst), Owned: t.Owned, After: b.taskChips(t.DependsOn),
	}
	for _, n := range t.DoD {
		row.DoD = append(row.DoD, b.criterionChip(n))
	}
	for _, c := range b.in.Doc.Commits {
		if slices.Contains(c.Tasks, t.Number) {
			row.Commits = append(row.Commits, b.commitChips([]int{c.Number})...)
		}
	}
	return row
}

func (b *builder) commits() []commitRow {
	rows := make([]commitRow, len(b.in.Doc.Commits))
	for i, c := range b.in.Doc.Commits {
		rows[i] = commitRow{
			Key: number(keyCommit, c.Number), Number: c.Number, Subject: c.Subject,
			State: b.states.Commits[c.Number], Tasks: b.taskChips(c.Tasks),
		}
	}
	return rows
}

func (b *builder) prs() []prRow {
	rows := make([]prRow, len(b.in.Doc.PRs))
	for i, p := range b.in.Doc.PRs {
		rows[i] = prRow{Key: number(keyPR, p.Number), Branch: p.Branch, Base: p.Base, ThisRun: p.ThisRun}
	}
	return rows
}

func (b *builder) matrix() matrixView {
	doc := b.in.Doc
	view := matrixView{Tasks: b.taskChips(taskNumbers(doc))}
	for _, c := range doc.Criteria {
		links := b.in.Links.Criteria[c.Number]
		row := matrixRow{
			Key: number(keyCriterion, c.Number), Number: c.Number, State: b.states.Proofs[c.Number],
			Text: strings.TrimSuffix(strings.SplitN(c.Text, sentenceBreak, 2)[0], "."), Specs: len(links.Specs),
		}
		files := map[string]bool{}
		for _, t := range doc.Tasks {
			owns := slices.Contains(links.Tasks, t.Number)
			row.Dots = append(row.Dots, owns)
			if owns {
				for _, f := range b.in.Links.Tasks[t.Number].Files {
					files[f] = true
				}
			}
		}
		row.Files = len(files)
		if links.Signal != nil {
			row.Signals = 1
		}
		view.Rows = append(view.Rows, row)
	}
	return view
}

func (b *builder) source() []sourceLine {
	bad := map[int]bool{}
	for _, p := range b.in.Problems {
		bad[p.Line] = true
	}
	lines := make([]sourceLine, len(b.in.Doc.Lines))
	for i, text := range b.in.Doc.Lines {
		lines[i] = sourceLine{N: i + 1, Text: text, Bad: bad[i+1]}
	}
	return lines
}

func (b *builder) signals() []signalRow {
	rows := make([]signalRow, len(b.in.Doc.Signals))
	for i, s := range b.in.Doc.Signals {
		rows[i] = signalRow{
			Key: b.signalKeys[s.Line], Signal: s.Signal, Source: s.Source, Query: s.Query,
			Breach: s.Breach, Window: s.Window, Panel: s.Panel, Criterion: []chip{b.criterionChip(s.Criterion)},
		}
	}
	return rows
}

func (b *builder) data() planData {
	doc := b.in.Doc
	d := planData{Graph: b.graph(), Labels: map[string]string{}, State: map[string]string{}, Diagrams: b.diagrams}
	if d.Diagrams == nil {
		d.Diagrams = []string{}
	}
	for _, c := range doc.Criteria {
		d.Labels[number(keyCriterion, c.Number)] = "Criterion " + itoa(c.Number)
		d.State[number(keyCriterion, c.Number)] = b.states.Proofs[c.Number]
	}
	for _, t := range doc.Tasks {
		d.Labels[number(keyTask, t.Number)] = "Task " + itoa(t.Number) + ": " + t.Name
	}
	for title, key := range b.specKeys {
		d.Labels[key] = title
	}
	for path, key := range b.fileKeys {
		d.Labels[key] = path
		d.State[key] = b.states.Files[path]
	}
	for _, c := range doc.Commits {
		d.Labels[number(keyCommit, c.Number)] = c.Subject
		d.State[number(keyCommit, c.Number)] = b.states.Commits[c.Number]
	}
	for _, s := range doc.Signals {
		d.Labels[b.signalKeys[s.Line]] = s.Signal
	}
	return d
}

func (b *builder) graph() map[string][]string {
	edges := map[string]map[string]bool{}
	link := func(x, y string) {
		for _, pair := range [][2]string{{x, y}, {y, x}} {
			if edges[pair[0]] == nil {
				edges[pair[0]] = map[string]bool{}
			}
			edges[pair[0]][pair[1]] = true
		}
	}
	for n, cl := range b.in.Links.Criteria {
		c := number(keyCriterion, n)
		for _, t := range cl.Tasks {
			link(c, number(keyTask, t))
		}
		for _, title := range cl.Specs {
			link(c, b.specKeys[title])
		}
		for _, k := range cl.Commits {
			link(c, number(keyCommit, k))
		}
		if cl.Signal != nil {
			link(c, b.signalKeys[cl.Signal.Line])
		}
	}
	for n, tl := range b.in.Links.Tasks {
		for _, f := range tl.Files {
			link(number(keyTask, n), b.fileKeys[f])
		}
	}
	for _, c := range b.in.Doc.Commits {
		for _, t := range c.Tasks {
			link(number(keyCommit, c.Number), number(keyTask, t))
		}
	}
	graph := make(map[string][]string, len(edges))
	for key, set := range edges {
		graph[key] = slices.Sorted(maps.Keys(set))
	}
	return graph
}

func number(prefix string, n int) string {
	return prefix + itoa(n)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func slug(path string) string {
	return nonAnchor.ReplaceAllString(path, "-")
}

func title(doc domain.Document) string {
	if len(doc.Lines) == 0 {
		return ""
	}
	return strings.TrimPrefix(strings.TrimPrefix(doc.Lines[0], titlePrefix), planPrefix)
}

func taskNumbers(doc domain.Document) []int {
	numbers := make([]int, len(doc.Tasks))
	for i, t := range doc.Tasks {
		numbers[i] = t.Number
	}
	return numbers
}

func sectionEnd(doc domain.Document, line int) int {
	for _, h := range doc.Headings {
		if h.Line > line {
			return h.Line
		}
	}
	return len(doc.Lines) + 1
}

func proseOf(doc domain.Document, heading string) []string {
	for _, h := range doc.Headings {
		if h.Level == headingLevel && h.Text == heading {
			return proseBetween(doc, h.Line+1, sectionEnd(doc, h.Line))
		}
	}
	return nil
}

func proseBetween(doc domain.Document, from, to int) []string {
	var out []string
	inFence := false
	for n := from; n < to && n <= len(doc.Lines); n++ {
		line := strings.TrimSpace(doc.Lines[n-1])
		switch {
		case strings.HasPrefix(line, fenceMark):
			inFence = !inFence
		case inFence, strings.HasPrefix(line, "|"), strings.HasPrefix(line, criteriaPrefix), strings.HasPrefix(line, trackPrefix):
		default:
			text := strings.TrimSpace(sketchLink.ReplaceAllString(strings.TrimPrefix(line, bulletPrefix), ""))
			if text != "" {
				out = append(out, text)
			}
		}
	}
	return out
}
