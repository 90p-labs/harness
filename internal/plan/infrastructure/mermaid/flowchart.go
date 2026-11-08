package mermaid

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

var ErrStateChart = errors.New("state diagram has no flowchart form")

const (
	flowchartHeader  = "flowchart"
	defaultDirection = "TB"
	rootScope        = "root"
	indent           = "    "
	subgraphKeyword  = "subgraph "
	endKeyword       = "end"
	notePrefix       = "note "
	pseudoStateOpen  = "<<"
	startNodeFormat  = `start_%s((" "))`
	endNodeFormat    = `stop_%s(((" ")))`
	labeledArrow     = `-->|"%s"|`
	plainArrow       = "-->"
	describedNode    = `%s["%s"]`
	linkStyleFormat  = "linkStyle %d %s"
)

var stateKinds = map[string]bool{"stateDiagram": true, "stateDiagram-v2": true}

var (
	stateAliasText = regexp.MustCompile(`^state\s+"([^"]*)"\s+as\s+(` + idPattern + `)$`)
	stateDescribe  = regexp.MustCompile(`^(` + idPattern + `)\s*:\s*([^:\s].*)$`)
)

type chart struct {
	direction string
	scopes    []string
	marks     shareddomain.Marks
	out       []string
	links     []string
	edges     int
}

func StateChart(body string) (string, error) {
	lines := strings.Split(body, "\n")
	h := headerLine(lines)
	if h < 0 || !stateKinds[strings.Fields(lines[h])[0]] {
		return body, nil
	}
	marks := shareddomain.ParseMarks(body)
	if len(marks.Nodes)+len(marks.Edges) == 0 {
		return body, nil
	}
	c := &chart{direction: defaultDirection, scopes: []string{rootScope}, marks: marks}
	for i := h + 1; i < len(lines); i++ {
		if err := c.add(i, strings.TrimSpace(lines[i])); err != nil {
			return "", fmt.Errorf("%w: line %d", ErrStateChart, i+1)
		}
	}
	out := append([]string{flowchartHeader + " " + c.direction}, c.out...)
	return strings.Join(append(out, c.links...), "\n"), nil
}

func headerLine(lines []string) int {
	for i, l := range lines {
		if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, commentPrefix) {
			return i
		}
	}
	return -1
}

func (c *chart) add(i int, text string) error {
	switch {
	case text == "":
	case strings.HasPrefix(text, commentPrefix), classDefinition.MatchString(text), stateClassUse.MatchString(text):
		c.emit(text)
	case strings.HasPrefix(text, notePrefix), strings.Contains(text, pseudoStateOpen), text == regionSeparator:
		return ErrStateChart
	case directionLine.MatchString(text):
		if len(c.scopes) == 1 {
			c.direction = strings.Fields(text)[1]
		} else {
			c.emit(text)
		}
	case text == closeBlock:
		if len(c.scopes) == 1 {
			return ErrStateChart
		}
		c.scopes = c.scopes[:len(c.scopes)-1]
		c.emit(endKeyword)
	case stateTransition.MatchString(text):
		c.transition(i, stateTransition.FindStringSubmatch(text))
	case stateAliasText.MatchString(text):
		m := stateAliasText.FindStringSubmatch(text)
		c.emit(fmt.Sprintf(describedNode, m[2], m[1]))
	case stateDescribe.MatchString(text):
		m := stateDescribe.FindStringSubmatch(text)
		c.emit(fmt.Sprintf(describedNode, m[1], strings.TrimSpace(m[2])))
	case stateDeclared.MatchString(text):
		c.declare(stateDeclared.FindStringSubmatch(text))
	default:
		c.emit(text)
	}
	return nil
}

func (c *chart) declare(m []string) {
	if m[2] == "" {
		c.emit(m[1])
		return
	}
	c.emit(subgraphKeyword + m[1])
	c.scopes = append(c.scopes, m[1])
}

func (c *chart) transition(i int, m []string) {
	arrow := plainArrow
	if label := strings.TrimSpace(m[3]); label != "" {
		arrow = fmt.Sprintf(labeledArrow, label)
	}
	c.emit(c.node(m[1], startNodeFormat) + " " + arrow + " " + c.node(m[2], endNodeFormat))
	if mark, ok := c.marks.Edges[i]; ok {
		c.links = append(c.links, fmt.Sprintf(linkStyleFormat, c.edges, shareddomain.MarkStyles[mark]))
	}
	c.edges++
}

func (c *chart) node(id, pseudoFormat string) string {
	if id != startMarker {
		return id
	}
	return fmt.Sprintf(pseudoFormat, c.scopes[len(c.scopes)-1])
}

func (c *chart) emit(line string) {
	c.out = append(c.out, strings.Repeat(indent, len(c.scopes))+line)
}
