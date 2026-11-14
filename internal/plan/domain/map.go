package domain

import (
	"regexp"
	"slices"
	"strings"

	shared "github.com/90p-labs/harness/internal/shared/domain"
)

const (
	HeadingDestination    = "Destination"
	HeadingNotes          = "Notes"
	HeadingDecisionsSoFar = "Decisions so far"
	HeadingUnspecified    = "Not yet specified"
	HeadingOutOfScope     = "Out of scope"
	HeadingTickets        = "Tickets"
)

const (
	FieldType      = "Type"
	FieldBlockedBy = "Blocked by"
	FieldStatus    = "Status"
	FieldClaimedBy = "Claimed by"
	FieldQuestion  = "Question"
	FieldAnswer    = "Answer"
)

const (
	TicketResearch  = "research"
	TicketPrototype = "prototype"
	TicketGrilling  = "grilling"
	TicketWork      = "work"
	TicketOpen      = "open"
	TicketClosed    = "closed"
)

const (
	FormTicketHeading = "T<n> slug"
	FormTicketBlocked = "none, T1, T1 and T3 or T1, T2 and T3"
	FormTicketField   = "- Field: value"

	listNone     = "none"
	questionMark = "?"
)

var (
	MapFields      = []string{FieldType, FieldBlockedBy, FieldStatus, FieldClaimedBy, FieldQuestion, FieldAnswer}
	TicketTypes    = []string{TicketResearch, TicketPrototype, TicketGrilling, TicketWork}
	TicketStatuses = []string{TicketOpen, TicketClosed}

	ticketHeading = regexp.MustCompile(`^(T\d+) ([a-z0-9]+(?:-[a-z0-9]+)*)$`)
	ticketID      = regexp.MustCompile(`^T\d+$`)
	strayHeading  = regexp.MustCompile(`^ {0,3}#{3,6}[ \t]+(T\d.*)$`)
)

type Map struct {
	Title       string
	TitleLine   int
	Destination []MapLine
	Notes       []MapLine
	Decisions   []MapLine
	Unspecified []MapLine
	OutOfScope  []MapLine
	Tickets     []Ticket
	Lines       []string
	Headings    []Heading
}

type MapLine struct {
	Text string
	Line int
}

type Ticket struct {
	ID         string
	Slug       string
	Type       string
	BlockedBy  []string
	Status     string
	Claim      string
	Question   string
	Answer     string
	Line       int
	FieldLines map[string]int
}

func IsMap(text string) bool {
	return hasHeading(text, HeadingDestination)
}

type mapParser struct {
	m        Map
	problems []Problem
	section  string
	ticket   int
	headed   bool // a ticket heading came in ## Tickets, good or bad
	seen     int  // the last line that the scan showed
}

func ParseMap(text string) (Map, []Problem) {
	p := &mapParser{m: Map{Lines: strings.Split(text, "\n")}, ticket: -1}
	open := scanOutsideFences(p.m.Lines, func(n int, line string) bool {
		p.fences(n - 1)
		p.seen = n
		p.strayTicket(line, n)
		switch {
		case strings.HasPrefix(line, "### "):
			p.heading(3, strings.TrimPrefix(line, "### "), n)
		case strings.HasPrefix(line, "## "):
			p.heading(2, strings.TrimPrefix(line, "## "), n)
		case strings.HasPrefix(line, "# ") && p.m.Title == "":
			p.m.Title = strings.TrimPrefix(line, "# ")
			p.m.TitleLine = n
		default:
			p.content(line, n)
		}
		return false
	})
	p.fences(len(p.m.Lines))
	if open != 0 && !p.fenceRefused() {
		p.fail(open, "code fence is never closed")
	}
	return p.m, p.problems
}

// fenceRefused tells whether a code fence could hide an item in the section.
// Only Destination, Notes and sections that the map does not name allow one.
func (p *mapParser) fenceRefused() bool {
	return slices.Contains([]string{HeadingDecisionsSoFar, HeadingUnspecified, HeadingOutOfScope, HeadingTickets}, p.section)
}

// strayTicket reports a heading of level 3 to 6 that reads as a ticket and
// stands outside ## Tickets, where the map reads no ticket.
func (p *mapParser) strayTicket(line string, n int) {
	if p.section == HeadingTickets {
		return
	}
	if p.section != "" && !slices.Contains(mapHeadings, p.section) {
		return
	}
	if m := strayHeading.FindStringSubmatch(line); m != nil {
		p.fail(n, "ticket heading "+m[1]+" stands outside ## "+HeadingTickets+", move it under ## "+HeadingTickets+" as ### "+FormTicketHeading+" or rename it")
	}
}

// fences reports each fence that opens in a refused section on the lines
// after the last shown line, up to the line last. The scan hides those lines.
func (p *mapParser) fences(last int) {
	if !p.fenceRefused() {
		return
	}
	opened := false
	for n := p.seen + 1; n <= last; n++ {
		line := strings.TrimRight(p.m.Lines[n-1], trailingSpace)
		switch {
		case opened:
			opened = !shared.IsFenceClose(line)
		case shared.IsFenceOpen(line):
			opened = true
			p.fail(n, "a code fence is not allowed under ## "+p.section)
		}
	}
}

func (p *mapParser) fail(n int, message string) {
	p.problems = append(p.problems, Problem{Line: n, Rule: RuleMapField, Message: message})
}

func (p *mapParser) heading(level int, text string, n int) {
	p.m.Headings = append(p.m.Headings, Heading{Level: level, Text: text, Line: n})
	p.ticket = -1
	if level == 2 {
		p.section = text
		p.headed = false
		return
	}
	if p.section != HeadingTickets {
		return
	}
	p.headed = true
	m := ticketHeading.FindStringSubmatch(text)
	if m == nil {
		p.fail(n, "ticket heading "+text+" must be ### "+FormTicketHeading)
		return
	}
	p.m.Tickets = append(p.m.Tickets, Ticket{ID: m[1], Slug: m[2], Line: n, FieldLines: map[string]int{}})
	p.ticket = len(p.m.Tickets) - 1
}

func (p *mapParser) content(line string, n int) {
	if strings.TrimSpace(line) == "" {
		return
	}
	entry := MapLine{Text: line, Line: n}
	switch p.section {
	case HeadingDestination:
		p.m.Destination = append(p.m.Destination, entry)
	case HeadingNotes:
		p.m.Notes = append(p.m.Notes, entry)
	case HeadingDecisionsSoFar:
		p.m.Decisions = append(p.m.Decisions, entry)
	case HeadingUnspecified:
		p.m.Unspecified = append(p.m.Unspecified, entry)
	case HeadingOutOfScope:
		p.m.OutOfScope = append(p.m.OutOfScope, entry)
	case HeadingTickets:
		p.ticketLine(line, n)
	}
}

func (p *mapParser) ticketLine(line string, n int) {
	bullet, ok := strings.CutPrefix(line, bulletMark)
	if p.ticket < 0 {
		if !p.headed {
			p.fail(n, "line "+line+" stands before the first ticket heading ### "+FormTicketHeading)
		}
		return
	}
	if !ok {
		p.fail(n, "line "+line+" must be "+FormTicketField)
		return
	}
	t := &p.m.Tickets[p.ticket]
	key, value, ok := strings.Cut(bullet, ":")
	switch {
	case !ok:
		p.fail(n, "line "+line+" must be "+FormTicketField)
		return
	case !slices.Contains(MapFields, key):
		p.fail(n, "ticket "+t.ID+" has the unknown field "+key+", the fields are "+strings.Join(MapFields, ", "))
		return
	case t.FieldLines[key] != 0:
		p.fail(n, "ticket "+t.ID+" has the field "+key+" twice")
		return
	}
	t.FieldLines[key] = n
	value = strings.TrimSpace(value)
	switch key {
	case FieldType:
		t.Type = value
	case FieldBlockedBy:
		ids, ok := ticketIDs(value)
		if !ok {
			p.fail(n, FieldBlockedBy+" must be "+FormTicketBlocked)
		}
		t.BlockedBy = ids
	case FieldStatus:
		t.Status = value
	case FieldClaimedBy:
		t.Claim = value
	case FieldQuestion:
		t.Question = value
	case FieldAnswer:
		t.Answer = value
	}
}

func ticketIDs(value string) ([]string, bool) {
	value = strings.TrimSuffix(value, sentenceStop)
	if value == listNone {
		return nil, true
	}
	if value == "" {
		return nil, false
	}
	ids := idList.Split(value, -1)
	if slices.ContainsFunc(ids, func(id string) bool { return !ticketID.MatchString(id) }) {
		return nil, false
	}
	return ids, true
}
