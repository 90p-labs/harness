package domain

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

type Mark string

const (
	MarkNew     Mark = "new"
	MarkChanged Mark = "changed"
	MarkRemoved Mark = "removed"
)

type grammar int

const (
	grammarFlow grammar = iota + 1
	grammarState
	grammarClass
	grammarER
)

const (
	commentPrefix    = "%%"
	frontMatter      = "---"
	notePrefix       = "note "
	linkStyleKeyword = "linkStyle"
	invisibleLink    = "~~~"
)

var ErrDiagramRefused = errors.New("diagram refused")

var errNoForm = errors.New("line has no accepted form")

var MarkStyles = map[Mark]string{
	MarkNew:     "stroke:#00b34e,stroke-width:3px,color:#00b34e",
	MarkChanged: "stroke:#e0a84a,stroke-width:3px,color:#e0a84a",
	MarkRemoved: "stroke:#e2001f,stroke-width:3px,color:#e2001f,stroke-dasharray:5 5",
}

var markOrder = []Mark{MarkNew, MarkChanged, MarkRemoved}

var trailingMarks = map[string]Mark{"+": MarkNew, "*": MarkChanged, "-": MarkRemoved}

var markableKinds = map[string]grammar{
	"flowchart": grammarFlow, "graph": grammarFlow,
	"stateDiagram": grammarState, "stateDiagram-v2": grammarState,
	"classDiagram": grammarClass, "classDiagram-v2": grammarClass,
	"erDiagram": grammarER,
}

var MarkableKindNames = slices.Sorted(maps.Keys(markableKinds))

const (
	reasonNoMark        = "no mark: end a line with + (new), * (changed) or - (removed)"
	reasonNoMarkInPlan  = "no mark: draw the marks with harness diagram"
	reasonAccessibility = "an accessibility line takes no mark"
	reasonInvisible     = "an invisible link takes no mark"
)

var blockEnds = map[string]bool{"}": true, "end": true}

var (
	attemptedMark = regexp.MustCompile(`^(.*\S)[ \t]+([^\w\s{}\[\]()"';])$`)
	quoted        = regexp.MustCompile(`"[^"]*"`)
	flowLink      = regexp.MustCompile(`--+[-xo>]|==+[=xo>]|-?\.+-[xo>]?|~~~`)
	pipeLabel     = regexp.MustCompile(`^\s*\|([^|]*)\|`)
	textLabel     = regexp.MustCompile(`(?:^|[^-=.])<?(?:--|==|-\.)\s+(\S.*?)\s*$`)
	textLabelSpan = regexp.MustCompile(`(?:^|[^-=.])<?(?:--|==|-\.)\s+(\S.*?)\s+(?:--+[-xo>]|==+[=xo>]|\.+-[xo>]?)`)
	accessibility = regexp.MustCompile(`^\s*acc(?:Title|Descr)\b`)
	accDescrOpen  = regexp.MustCompile(`^accDescr\s*\{[^}]*$`)
	linkStyleLine = regexp.MustCompile(`^linkStyle\s+(\d+(?:\s*,\s*\d+)*)\s+(\S.*?);?$`)
	linkEdge      = regexp.MustCompile(`\s(?:-->|<--|--\|>|<\|--|\.\.>|<\.\.|\.\.\|>|<\|\.\.|\*--|--\*|o--|--o|--|\.\.)\s`)
	erRelation    = regexp.MustCompile(`^[\w-]+\s+[|}o][|o](?:--|\.\.)[|o][|{]\s+[\w-]+`)
	erRelLabel    = regexp.MustCompile(`^(.*?\S)\s*:\s*(.*)$`)
	erComment     = regexp.MustCompile(`"[^"]*"$`)
	erEntity      = regexp.MustCompile(`^[\w-]+(?:\s*\[.*\])?(?:\s*\{)?$`)

	nodeLines = map[grammar]*regexp.Regexp{
		grammarFlow:  regexp.MustCompile(`^([\w-]+)\s*(?:[\[\(\{>].*)?$`),
		grammarState: regexp.MustCompile(`^(?:state\s+(?:"[^"]*"\s+as\s+)?)?([\w-]+)(?:\s*:.*|\s*\{)?$`),
		grammarClass: regexp.MustCompile(`^class\s+"?([\w.-]+)"?(?:\s*\{)?$`),
	}

	classDefLine  = regexp.MustCompile(`^classDef\s+(\S+)\s+(.*?);?$`)
	classUseLine  = regexp.MustCompile(`^class\s+\S+\s+([A-Za-z_]\w*);?$`)
	cssClassLine  = regexp.MustCompile(`^cssClass\s+"[^"]*"\s+([A-Za-z_]\w*);?$`)
	shorthandUse  = regexp.MustCompile(`:::([A-Za-z_]\w*)`)
	labelMarkText = regexp.MustCompile(`\((?:new|changed|removed)\)`)
)

func DiagramKind(body string) string {
	lines := strings.Split(body, "\n")
	if h := headerIndex(lines); h >= 0 {
		return firstWord(lines[h])
	}
	return ""
}

func MarkDiagram(input string) (string, error) {
	lines := strings.Split(strings.TrimRight(strings.ReplaceAll(input, "\r\n", "\n"), " \t\n"), "\n")
	h := headerIndex(lines)
	if h < 0 {
		return "", refuse(1, "no diagram header")
	}
	kind := firstWord(lines[h])
	g, ok := markableKinds[kind]
	if !ok {
		return "", refuse(h+1, "kind "+kind+" takes no mark; marked kinds: "+strings.Join(MarkableKindNames, ", ")+"; paste another kind as it is")
	}

	type markedID struct {
		id   string
		mark Mark
	}
	type markedLinks struct {
		first, count int
		mark         Mark
	}
	var (
		body       []string
		ids        []markedID
		links      []markedLinks
		used       = map[Mark]bool{}
		seen       = map[string]bool{}
		inBlock    bool
		anyMark    bool
		nextLink   int
		inAccDescr bool
		countLink  = g == grammarFlow
	)
	if countLink {
		nextLink = len(headerLinkMarks(lines[h]))
	}
	for i := h + 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		text := strings.TrimSpace(line)
		var skip bool
		if skip, inAccDescr = accDescrBlock(text, inAccDescr); skip {
			body = append(body, line)
			continue
		}
		m := attemptedMark.FindStringSubmatch(line)
		if m == nil || isText(g, text) {
			if countLink {
				nextLink += flowLinkCount(line)
			}
			body = append(body, line)
			if strings.HasSuffix(text, "{") {
				inBlock = true
			} else if text == "}" {
				inBlock = false
			}
			continue
		}
		mark, ok := trailingMarks[m[2]]
		if !ok {
			return "", refuse(i+1, "unknown mark "+m[2])
		}
		marked, id, err := markLine(g, m[1], mark, inBlock)
		if err != nil {
			return "", refuse(i+1, err.Error())
		}
		if id != "" {
			if seen[id] {
				return "", refuse(i+1, id+" has a second mark")
			}
			seen[id] = true
			ids = append(ids, markedID{id, mark})
			used[mark] = true
		}
		if countLink {
			n := flowLinkCount(m[1])
			if id == "" {
				links = append(links, markedLinks{nextLink, n, mark})
			}
			nextLink += n
		}
		anyMark = true
		body = append(body, marked)
	}
	if !anyMark {
		return "", refuse(h+1, reasonNoMark)
	}

	out := make([]string, 0, len(lines)+len(markOrder)+len(ids)+len(links))
	for _, l := range lines[:h+1] {
		out = append(out, strings.TrimRight(l, " \t"))
	}
	if g != grammarER {
		for _, m := range markOrder {
			if used[m] {
				out = append(out, "    classDef "+string(m)+" "+MarkStyles[m])
			}
		}
	}
	out = append(out, body...)
	for _, n := range ids {
		if g == grammarClass {
			out = append(out, fmt.Sprintf("    cssClass %q %s", n.id, n.mark))
		} else {
			out = append(out, fmt.Sprintf("    class %s %s", n.id, n.mark))
		}
	}
	for _, l := range links {
		indexes := make([]string, l.count)
		for k := range indexes {
			indexes[k] = strconv.Itoa(l.first + k)
		}
		out = append(out, fmt.Sprintf("    %s %s %s", linkStyleKeyword, strings.Join(indexes, ","), MarkStyles[l.mark]))
	}
	return strings.Join(out, "\n"), nil
}

func CheckMarks(body string) []string {
	lines := strings.Split(body, "\n")
	h := headerIndex(lines)
	if h < 0 {
		return nil
	}
	g, ok := markableKinds[firstWord(lines[h])]
	if !ok {
		return nil
	}
	var problems, usedFixed []string
	add := func(p string) {
		if !slices.Contains(problems, p) {
			problems = append(problems, p)
		}
	}
	type styledLink struct {
		written string
		index   int
		mark    Mark
	}
	var (
		defined    = map[string]bool{}
		linkMarks  = map[int]Mark{}
		styled     []styledLink
		nextLink   int
		marked     bool
		inAccDescr bool
	)
	if g == grammarFlow {
		for _, mark := range headerLinkMarks(lines[h]) {
			linkMarks[nextLink] = mark
			nextLink++
		}
	}
	for i := h + 1; i < len(lines); i++ {
		text := strings.TrimSpace(lines[i])
		var skip bool
		if skip, inAccDescr = accDescrBlock(text, inAccDescr); skip || isText(g, text) {
			continue
		}
		if m := attemptedMark.FindStringSubmatch(text); m != nil {
			add(fmt.Sprintf("line %d keeps the raw mark %s", i+1, m[2]))
		}
		if g == grammarFlow {
			for _, mark := range flowLinkMarks(text) {
				linkMarks[nextLink] = mark
				nextLink++
			}
			if m := linkStyleLine.FindStringSubmatch(text); m != nil {
				if mark, ok := markOfStyle(m[2]); ok {
					for _, index := range strings.Split(m[1], ",") {
						written := strings.TrimSpace(index)
						at, err := strconv.Atoi(written)
						if err != nil {
							at = -1
						}
						styled = append(styled, styledLink{written, at, mark})
					}
				}
				continue
			}
		}
		def := classDefLine.FindStringSubmatch(text)
		uses := classUses(text)
		if g == grammarER && (def != nil || len(uses) > 0) {
			add("erDiagram takes no classDef or class line")
			continue
		}
		if def != nil {
			style, fixed := MarkStyles[Mark(def[1])]
			switch {
			case !fixed:
				add("classDef " + def[1] + " has another name")
			case strings.TrimSpace(def[2]) != style:
				defined[def[1]] = true
				add("classDef " + def[1] + " has another style")
			default:
				defined[def[1]] = true
			}
			continue
		}
		for _, name := range uses {
			marked = true
			if _, fixed := MarkStyles[Mark(name)]; fixed {
				usedFixed = append(usedFixed, name)
			} else {
				add("class " + name + " is not a mark")
			}
		}
		if labelMarkText.MatchString(text) && isEdge(g, text) {
			marked = true
		}
	}
	for _, name := range usedFixed {
		if !defined[name] {
			add("classDef " + name + " is missing")
		}
	}
	for _, s := range styled {
		if linkMarks[s.index] != s.mark {
			add(fmt.Sprintf("%s %s is not a %s link", linkStyleKeyword, s.written, s.mark))
		}
	}
	if !marked {
		add(reasonNoMarkInPlan)
	}
	return problems
}

func markOfStyle(style string) (Mark, bool) {
	for m, s := range MarkStyles {
		if s == style {
			return m, true
		}
	}
	return "", false
}

type Marks struct {
	Nodes map[string]Mark
	Edges map[int]Mark
}

func ParseMarks(body string) Marks {
	marks := Marks{Nodes: map[string]Mark{}, Edges: map[int]Mark{}}
	lines := strings.Split(body, "\n")
	h := headerIndex(lines)
	if h < 0 {
		return marks
	}
	g, ok := markableKinds[firstWord(lines[h])]
	if !ok {
		return marks
	}
	for i := h + 1; i < len(lines); i++ {
		text := strings.TrimSpace(lines[i])
		if isText(g, text) {
			continue
		}
		if ids, mark, ok := classAssignment(text); ok {
			if _, fixed := MarkStyles[mark]; fixed {
				for _, id := range ids {
					marks.Nodes[id] = mark
				}
			}
			continue
		}
		if m := labelMarkText.FindString(text); m != "" && isEdge(g, text) {
			marks.Edges[i] = Mark(strings.Trim(m, "()"))
		}
	}
	return marks
}

func classAssignment(text string) (ids []string, mark Mark, ok bool) {
	m := classUseLine.FindStringSubmatch(text)
	if m == nil {
		m = cssClassLine.FindStringSubmatch(text)
	}
	if m == nil {
		return nil, "", false
	}
	return strings.Split(strings.Trim(strings.Fields(text)[1], `"`), ","), Mark(m[1]), true
}

func classUses(text string) []string {
	if m := classUseLine.FindStringSubmatch(text); m != nil {
		return m[1:]
	}
	if m := cssClassLine.FindStringSubmatch(text); m != nil {
		return m[1:]
	}
	var names []string
	for _, m := range shorthandUse.FindAllStringSubmatch(text, -1) {
		names = append(names, m[1])
	}
	return names
}

func isEdge(g grammar, text string) bool {
	switch g {
	case grammarFlow:
		return flowLinkCount(text) > 0
	case grammarER:
		return true
	}
	return linkEdge.MatchString(text)
}

func isText(g grammar, text string) bool {
	hasNotes := g == grammarState || g == grammarClass
	return strings.HasPrefix(text, commentPrefix) || hasNotes && strings.HasPrefix(text, notePrefix)
}

func markLine(g grammar, content string, m Mark, inBlock bool) (line, id string, err error) {
	text := strings.TrimSpace(content)
	switch {
	case accessibility.MatchString(text):
		return "", "", errors.New(reasonAccessibility)
	case blockEnds[text]:
		return "", "", errors.New("a block end takes no mark")
	case g == grammarER:
		line, err = markER(content, text, m, inBlock)
		return line, "", err
	case g == grammarFlow:
		if line, ok, err := markFlowEdge(content, m); ok || err != nil {
			return line, "", err
		}
	default:
		if line, ok := markLinkEdge(content, m); ok {
			return line, "", nil
		}
	}
	if n := nodeLines[g].FindStringSubmatch(text); n != nil {
		return content, n[1], nil
	}
	return "", "", errNoForm
}

func markFlowEdge(content string, m Mark) (string, bool, error) {
	masked := maskFlow(content)
	links := flowLink.FindAllStringIndex(masked, -1)
	switch {
	case len(links) == 0:
		return "", false, nil
	case len(links) > 1:
		return "", false, errors.New("a chain edge takes no mark")
	}
	start, end := links[0][0], links[0][1]
	if masked[start:end] == invisibleLink {
		return "", false, errors.New(reasonInvisible)
	}
	if l := pipeLabel.FindStringSubmatchIndex(masked[end:]); l != nil {
		return withLabel(content, end+l[2], end+l[3], m), true, nil
	}
	if l := textLabel.FindStringSubmatchIndex(masked[:start]); l != nil {
		return withLabel(content, l[2], l[3], m), true, nil
	}
	return content[:end] + "|" + quotedLabel("", m) + "|" + content[end:], true, nil
}

func markLinkEdge(content string, m Mark) (string, bool) {
	loc := linkEdge.FindStringIndex(content)
	if loc == nil {
		return "", false
	}
	if strings.Contains(content[loc[1]:], ":") {
		return content + " " + labelMark(m), true
	}
	return content + " : " + labelMark(m), true
}

func markER(content, text string, m Mark, inBlock bool) (string, error) {
	switch {
	case inBlock:
		if l := erComment.FindStringIndex(content); l != nil {
			return withLabel(content, l[0], l[1], m), nil
		}
		return content + " " + quotedLabel("", m), nil
	case erRelation.MatchString(text):
		if l := erRelLabel.FindStringSubmatch(content); l != nil {
			return l[1] + " : " + quotedLabel(l[2], m), nil
		}
		return content + " : " + quotedLabel("", m), nil
	case erEntity.MatchString(text):
		return "", errors.New("an entity takes no mark")
	}
	return "", errNoForm
}

func maskFlow(line string) string {
	masked := maskQuoted(line)
	if at := strings.Index(masked, commentPrefix); at >= 0 {
		masked = masked[:at] + strings.Repeat("_", len(masked)-at)
	}
	b := []byte(masked)
	depth, inPipe := 0, false
	for i, c := range b {
		switch c {
		case '[', '(', '{', '>':
			opens := c != '>' || depth == 0 && followsNodeID(masked, i)
			if !inPipe && opens {
				depth++
				continue
			}
		case ']', ')', '}':
			if !inPipe && depth > 0 {
				depth--
				continue
			}
		case '|':
			if depth == 0 {
				inPipe = !inPipe
				continue
			}
		}
		if depth > 0 || inPipe {
			b[i] = '_'
		}
	}
	for _, l := range textLabelSpan.FindAllSubmatchIndex(b, -1) {
		for i := l[2]; i < l[3]; i++ {
			b[i] = '_'
		}
	}
	return string(b)
}

func followsNodeID(masked string, at int) bool {
	start := at
	for start > 0 && isWordByte(masked[start-1]) {
		start--
	}
	if start == at {
		return false
	}
	before := strings.TrimRight(masked[:start], " \t")
	if before == "" || strings.HasSuffix(before, "&") || strings.HasSuffix(before, ";") {
		return true
	}
	links := flowLink.FindAllStringIndex(before, -1)
	return len(links) > 0 && links[len(links)-1][1] == len(before)
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func headerLinkMarks(header string) []Mark {
	_, rest, found := strings.Cut(header, ";")
	if !found {
		return nil
	}
	return flowLinkMarks(rest)
}

func accDescrBlock(text string, inside bool) (skip, next bool) {
	if inside {
		return true, !strings.Contains(text, "}")
	}
	open := accDescrOpen.MatchString(text)
	return open, open
}

func flowLinkCount(line string) int {
	return len(flowLinkMarks(line))
}

func flowLinkMarks(line string) []Mark {
	if accessibility.MatchString(line) {
		return nil
	}
	var marks []Mark
	masked := maskFlow(line)
	base := 0
	for _, statement := range strings.Split(masked, ";") {
		var sides []int
		var labels []Mark
		start := 0
		for _, l := range flowLink.FindAllStringIndex(statement, -1) {
			sides = append(sides, strings.Count(statement[start:l[0]], "&")+1)
			from, to := 0, 0
			if t := textLabel.FindStringSubmatchIndex(statement[start:l[0]]); t != nil {
				from, to = base+start+t[2], base+start+t[3]
			}
			start = l[1]
			if p := pipeLabel.FindStringSubmatchIndex(statement[start:]); p != nil {
				from, to = base+start+p[2], base+start+p[3]
				start += p[1]
			}
			labels = append(labels, markInLabel(line[from:to]))
		}
		base += len(statement) + 1
		if len(sides) == 0 {
			continue
		}
		sides = append(sides, strings.Count(statement[start:], "&")+1)
		for k := 1; k < len(sides); k++ {
			for range sides[k-1] * sides[k] {
				marks = append(marks, labels[k-1])
			}
		}
	}
	return marks
}

func markInLabel(label string) Mark {
	return Mark(strings.Trim(labelMarkText.FindString(label), "()"))
}

func maskQuoted(s string) string {
	return quoted.ReplaceAllStringFunc(s, func(q string) string {
		return `"` + strings.Repeat("_", len(q)-2) + `"`
	})
}

func withLabel(content string, from, to int, m Mark) string {
	return content[:from] + quotedLabel(content[from:to], m) + content[to:]
}

// Mermaid refuses parentheses in an unquoted flowchart edge label.
func quotedLabel(existing string, m Mark) string {
	return `"` + strings.TrimSpace(strings.Trim(existing, `"`)+" "+labelMark(m)) + `"`
}

func labelMark(m Mark) string {
	return "(" + string(m) + ")"
}

func headerIndex(lines []string) int {
	h := firstStatement(lines, 0)
	if h < 0 || strings.TrimSpace(lines[h]) != frontMatter {
		return h
	}
	end := slices.IndexFunc(lines[h+1:], func(l string) bool { return strings.TrimSpace(l) == frontMatter })
	if end < 0 {
		return h
	}
	return firstStatement(lines, h+end+2)
}

func firstStatement(lines []string, from int) int {
	for i := from; i < len(lines); i++ {
		if t := strings.TrimSpace(lines[i]); t != "" && !strings.HasPrefix(t, commentPrefix) {
			return i
		}
	}
	return -1
}

func firstWord(line string) string {
	return strings.Fields(line)[0]
}

func refuse(line int, reason string) error {
	return fmt.Errorf("%w: line %d: %s", ErrDiagramRefused, line, reason)
}
