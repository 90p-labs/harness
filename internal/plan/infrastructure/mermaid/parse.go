package mermaid

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jorge07/harness/internal/plan/domain"
)

const (
	commentPrefix   = "%%"
	reservedID      = "root"
	closeBlock      = "}"
	regionSeparator = "--"
	endNote         = "end note"
	startMarker     = "[*]"
)

const (
	idPattern       = `[A-Za-z_][\w.-]*`
	stylePattern    = `\S+`
	classRelations  = `<\|--|--\|>|<\|\.\.|\.\.\|>|\*--|--\*|o--|--o|-->|<--|\.\.>|<\.\.|--|\.\.`
	stateEndpoint   = `(\[\*\]|` + idPattern + `)(?::::` + idPattern + `)?`
	stateKind       = `(?:\s+<<(?:choice|fork|join)>>)?`
	cardinalityPart = `(?:"[^"]*"\s+)?`
)

var (
	stateTransition = regexp.MustCompile(`^` + stateEndpoint + `\s*-->\s*` + stateEndpoint + `(?:\s*:\s*(.*))?$`)
	stateBareID     = regexp.MustCompile(`^` + idPattern + `$`)
	stateDescribed  = regexp.MustCompile(`^(` + idPattern + `)\s*:\s*[^:\s].*$`)
	stateAlias      = regexp.MustCompile(`^state\s+"[^"]*"\s+as\s+(` + idPattern + `)$`)
	stateDeclared   = regexp.MustCompile(`^state\s+(` + idPattern + `)` + stateKind + `(\s*\{)?$`)
	stateNote       = regexp.MustCompile(`^note\s+(?:left|right)\s+of\s+(` + idPattern + `)(\s*:.*)?$`)
	stateClassUse   = regexp.MustCompile(`^class\s+(` + idPattern + `(?:,` + idPattern + `)*)\s+` + stylePattern + `$`)
	classDefinition = regexp.MustCompile(`^classDef\s+` + idPattern + `\s+` + stylePattern + `.*$`)
	directionLine   = regexp.MustCompile(`^direction\s+(?:TB|BT|LR|RL)$`)

	classDeclared = regexp.MustCompile(`^class\s+(` + idPattern + `)(\s*\{)?$`)
	classCSS      = regexp.MustCompile(`^cssClass\s+"([^"]+)"\s+` + stylePattern + `$`)
	classRelation = regexp.MustCompile(`^(` + idPattern + `)\s+` + cardinalityPart + `(?:` + classRelations + `)\s+` + cardinalityPart + `(` + idPattern + `)(?:\s*:\s*(.*))?$`)
)

type Parser struct{}

var _ domain.DiagramParser = Parser{}

type grammar interface {
	statement(line string) error
	finish() error
}

func (Parser) Parse(kind, body string) error {
	var g grammar
	switch kind {
	case domain.DiagramState:
		g = &stateGrammar{}
	case domain.DiagramClass:
		g = &classGrammar{}
	default:
		return fmt.Errorf("unsupported kind %q", kind)
	}
	lines := strings.Split(body, "\n")
	headerSeen := false
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, commentPrefix) {
			continue
		}
		if !headerSeen {
			if line != kind {
				return atLine(i+1, fmt.Errorf("header must be %s", kind))
			}
			headerSeen = true
			continue
		}
		if err := g.statement(line); err != nil {
			return atLine(i+1, err)
		}
	}
	if err := g.finish(); err != nil {
		return atLine(len(lines), err)
	}
	return nil
}

func atLine(n int, err error) error {
	return fmt.Errorf("line %d: %w", n, err)
}

func checkID(id string) error {
	if id == reservedID {
		return fmt.Errorf("the id %q is reserved", reservedID)
	}
	return nil
}

func checkLabel(label string) error {
	if strings.Contains(label, ":") {
		return errors.New("second colon in the label")
	}
	return nil
}

func checkIDs(ids ...string) error {
	for _, id := range ids {
		if err := checkID(id); err != nil {
			return err
		}
	}
	return nil
}

type stateGrammar struct {
	depth  int
	inNote bool
}

func (g *stateGrammar) statement(line string) error {
	if g.inNote {
		g.inNote = line != endNote
		return nil
	}
	switch {
	case line == closeBlock:
		if g.depth == 0 {
			return errors.New("unexpected }")
		}
		g.depth--
		return nil
	case directionLine.MatchString(line), classDefinition.MatchString(line):
		return nil
	case line == regionSeparator && g.depth > 0:
		return nil
	case strings.HasPrefix(line, "note "):
		return g.note(line)
	case stateDescribed.MatchString(line):
		return checkID(stateDescribed.FindStringSubmatch(line)[1])
	case strings.Contains(line, "->"):
		return stateEdge(line)
	}
	if m := stateClassUse.FindStringSubmatch(line); m != nil {
		return checkIDs(strings.Split(m[1], ",")...)
	}
	if m := stateAlias.FindStringSubmatch(line); m != nil {
		return checkID(m[1])
	}
	if m := stateDeclared.FindStringSubmatch(line); m != nil {
		if m[2] != "" {
			g.depth++
		}
		return checkID(m[1])
	}
	if stateBareID.MatchString(line) {
		return checkID(line)
	}
	return fmt.Errorf("unknown statement %q", line)
}

func (g *stateGrammar) note(line string) error {
	m := stateNote.FindStringSubmatch(line)
	if m == nil {
		return fmt.Errorf("bad note %q", line)
	}
	g.inNote = m[2] == ""
	return checkID(m[1])
}

func stateEdge(line string) error {
	m := stateTransition.FindStringSubmatch(line)
	if m == nil {
		return fmt.Errorf("bad edge %q", line)
	}
	if err := checkIDs(m[1], m[2]); err != nil {
		return err
	}
	return checkLabel(m[3])
}

func (g *stateGrammar) finish() error {
	if g.inNote {
		return errors.New("unclosed note")
	}
	if g.depth > 0 {
		return errors.New("unclosed state block")
	}
	return nil
}

type classGrammar struct {
	inBody bool
}

func (g *classGrammar) statement(line string) error {
	if g.inBody {
		g.inBody = line != closeBlock
		return nil
	}
	if directionLine.MatchString(line) || classDefinition.MatchString(line) {
		return nil
	}
	if m := classCSS.FindStringSubmatch(line); m != nil {
		return checkIDs(strings.Split(m[1], ",")...)
	}
	if m := classDeclared.FindStringSubmatch(line); m != nil {
		g.inBody = m[2] != ""
		return checkID(m[1])
	}
	if m := classRelation.FindStringSubmatch(line); m != nil {
		if err := checkIDs(m[1], m[2]); err != nil {
			return err
		}
		return checkLabel(m[3])
	}
	return fmt.Errorf("not a class, classDef, cssClass, direction or relation: %q", line)
}

func (g *classGrammar) finish() error {
	if g.inBody {
		return errors.New("unclosed class body")
	}
	return nil
}
