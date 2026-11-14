package domain

import (
	"regexp"
	"slices"
	"strings"

	shared "github.com/90p-labs/harness/internal/shared/domain"
)

var (
	mapHeadings  = []string{HeadingDestination, HeadingNotes, HeadingDecisionsSoFar, HeadingUnspecified, HeadingOutOfScope, HeadingTickets}
	mapRequired  = []string{FieldType, FieldBlockedBy, FieldStatus, FieldQuestion}
	decisionLine = regexp.MustCompile(`^(T\d+): \S`)
	mapTitle     = regexp.MustCompile(`^Map: \S`)
)

func CheckMap(m Map) []Problem {
	var problems []Problem
	add := reporter(&problems)
	checkMapHeadings(m, add)
	checkMapTickets(m, add)
	checkMapLists(m, add)
	checkMapIndex(m, add)
	checkSentences(Document{Lines: m.Lines}, add)
	SortProblems(problems)
	return problems
}

// Frontier returns the open tickets that have no claim and no open blocker, in file order.
func (m Map) Frontier() []Ticket {
	closed := map[string]bool{}
	for _, t := range m.Tickets {
		closed[t.ID] = t.Status == TicketClosed
	}
	var free []Ticket
	for _, t := range m.Tickets {
		if t.Status != TicketOpen || t.Claim != "" {
			continue
		}
		if slices.ContainsFunc(t.BlockedBy, func(id string) bool { return !closed[id] }) {
			continue
		}
		free = append(free, t)
	}
	return free
}

// Clear is true when no ticket is open and nothing is left unspecified.
func (m Map) Clear() bool {
	if slices.ContainsFunc(m.Tickets, func(t Ticket) bool { return t.Status == TicketOpen }) {
		return false
	}
	return len(m.Unspecified) == 1 && isNoneLine(m.Unspecified[0].Text)
}

// isNoneLine is true for the list line `- none`, with any space after the bullet.
func isNoneLine(text string) bool {
	item, ok := strings.CutPrefix(text, bulletMark)
	return ok && strings.EqualFold(strings.TrimSuffix(strings.TrimSpace(item), sentenceStop), listNone)
}

func checkMapHeadings(m Map, add report) {
	doc := Document{Headings: m.Headings}
	if !mapTitle.MatchString(m.Title) {
		add(lineOr(m.TitleLine, 1), RuleMapField, "the title line must be `# Map: title`, with a title")
	}
	if line := headingLine(doc, 2, HeadingDestination); line != 0 && len(m.Destination) == 0 {
		add(line, RuleMapField, "## %s needs text under it", HeadingDestination)
	}
	for _, text := range mapHeadings {
		if text != HeadingNotes && headingLine(doc, 2, text) == 0 {
			add(1, RuleHeadingMissing, "heading ## %s is missing", text)
		}
	}
	last := -1
	for _, h := range m.Headings {
		if h.Level != 2 {
			continue
		}
		i := slices.Index(mapHeadings, h.Text)
		if i < 0 {
			add(h.Line, RuleMapField, "heading ## %s is not in the map format, the headings are %s", h.Text, strings.Join(mapHeadings, ", "))
			continue
		}
		if i <= last {
			add(h.Line, RuleHeadingMissing, "heading ## %s is out of order, the order is %s", h.Text, strings.Join(mapHeadings, ", "))
			continue
		}
		last = i
	}
}

func checkMapTickets(m Map, add report) {
	byID := make(map[string]Ticket, len(m.Tickets))
	for _, t := range m.Tickets {
		if _, ok := byID[t.ID]; ok {
			add(t.Line, RuleMapField, "ticket id %s is used twice", t.ID)
			continue
		}
		byID[t.ID] = t
	}
	for _, t := range m.Tickets {
		checkTicketFields(t, add)
		checkTicketQuestion(t, add)
		for _, b := range t.BlockedBy {
			line := t.FieldLines[FieldBlockedBy]
			_, known := byID[b]
			switch {
			case b == t.ID:
				add(line, RuleMapBlocked, "ticket %s is blocked by itself", t.ID)
			case !known:
				add(line, RuleMapBlocked, "ticket %s is blocked by %s, which is not a ticket of the map", t.ID, b)
			case t.Status == TicketClosed && byID[b].Status == TicketOpen:
				add(line, RuleMapBlocked, "closed ticket %s is blocked by %s, which is still open", t.ID, b)
			}
		}
	}
	checkTicketCycles(m.Tickets, byID, add)
}

// checkTicketOrder reports a field that comes after a field that the format puts later.
func checkTicketOrder(t Ticket, add report) {
	var fields []string
	for _, f := range MapFields {
		if t.FieldLines[f] != 0 {
			fields = append(fields, f)
		}
	}
	slices.SortFunc(fields, func(a, b string) int { return t.FieldLines[a] - t.FieldLines[b] })
	top := -1
	for _, f := range fields {
		rank := slices.Index(MapFields, f)
		if rank < top {
			add(t.FieldLines[f], RuleMapField, "ticket %s has %s after %s, the order is %s", t.ID, f, MapFields[top], strings.Join(MapFields, ", "))
			continue
		}
		top = rank
	}
}

func checkTicketFields(t Ticket, add report) {
	checkTicketOrder(t, add)
	has := func(f string) bool { return t.FieldLines[f] != 0 }
	for _, f := range mapRequired {
		if !has(f) {
			add(t.Line, RuleMapField, "ticket %s has no %s: line", t.ID, f)
		}
	}
	if has(FieldType) && !slices.Contains(TicketTypes, t.Type) {
		add(t.FieldLines[FieldType], RuleMapField, "%s must be %s", FieldType, strings.Join(TicketTypes, ", "))
	}
	if has(FieldStatus) && !slices.Contains(TicketStatuses, t.Status) {
		add(t.FieldLines[FieldStatus], RuleMapField, "%s must be %s", FieldStatus, strings.Join(TicketStatuses, ", "))
	}
	if has(FieldClaimedBy) && t.Claim == "" {
		add(t.FieldLines[FieldClaimedBy], RuleMapField, "%s needs a name", FieldClaimedBy)
	} else if has(FieldClaimedBy) && strings.EqualFold(strings.TrimSuffix(t.Claim, "."), listNone) {
		add(t.FieldLines[FieldClaimedBy], RuleMapField, "%s: %s is not a name: remove the line to free the ticket", FieldClaimedBy, t.Claim)
	}
	switch t.Status {
	case TicketClosed:
		if t.Answer == "" {
			add(t.Line, RuleMapField, "closed ticket %s needs an %s: line with text", t.ID, FieldAnswer)
		}
		if has(FieldClaimedBy) {
			add(t.FieldLines[FieldClaimedBy], RuleMapField, "closed ticket %s has a %s: line", t.ID, FieldClaimedBy)
		}
	case TicketOpen:
		if has(FieldAnswer) {
			add(t.FieldLines[FieldAnswer], RuleMapField, "open ticket %s has an %s: line", t.ID, FieldAnswer)
		}
	}
}

func checkTicketQuestion(t Ticket, add report) {
	line := t.FieldLines[FieldQuestion]
	question := shared.MaskCodeSpans(t.Question)
	switch {
	case line == 0:
		return
	case strings.Count(question, questionMark) > 1:
		add(line, RuleMapQuestion, "ticket %s asks more than one question: keep one", t.ID)
	case !strings.HasSuffix(question, questionMark):
		add(line, RuleMapQuestion, "%s of ticket %s must end with a question mark", FieldQuestion, t.ID)
	}
}

func checkTicketCycles(tickets []Ticket, byID map[string]Ticket, add report) {
	reach := make(map[string]map[string]bool, len(tickets))
	for _, t := range tickets {
		reach[t.ID] = reachable(byID, t.ID)
	}
	reported := map[string]bool{}
	for _, t := range tickets {
		if reported[t.ID] || !reach[t.ID][t.ID] {
			continue
		}
		var ring []string
		for _, u := range tickets {
			if !slices.Contains(ring, u.ID) && (u.ID == t.ID || reach[t.ID][u.ID] && reach[u.ID][t.ID]) {
				ring = append(ring, u.ID)
				reported[u.ID] = true
			}
		}
		add(lineOr(t.FieldLines[FieldBlockedBy], t.Line), RuleMapBlocked, "tickets %s block each other in a cycle", strings.Join(ring, ", "))
	}
}

// reachable returns the tickets that a walk over the blockers of from reaches.
// A ticket that blocks itself is a separate problem, so its own edge is skipped.
func reachable(byID map[string]Ticket, from string) map[string]bool {
	seen := map[string]bool{}
	stack := []string{from}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, b := range byID[id].BlockedBy {
			if _, ok := byID[b]; !ok || b == id || seen[b] {
				continue
			}
			seen[b] = true
			stack = append(stack, b)
		}
	}
	return seen
}

func checkMapLists(m Map, add report) {
	doc := Document{Headings: m.Headings}
	lists := []struct {
		heading string
		lines   []MapLine
	}{
		{HeadingDecisionsSoFar, m.Decisions},
		{HeadingUnspecified, m.Unspecified},
		{HeadingOutOfScope, m.OutOfScope},
	}
	for _, list := range lists {
		heading := headingLine(doc, 2, list.heading)
		if heading == 0 {
			continue
		}
		if len(list.lines) == 0 {
			add(heading, RuleMapField, "## %s needs `- %s` or one bullet on each line", list.heading, listNone)
		}
		for _, l := range list.lines {
			switch {
			case !strings.HasPrefix(l.Text, bulletMark):
				add(l.Line, RuleMapField, "a line of ## %s must be a bullet", list.heading)
			case isNoneLine(l.Text) && len(list.lines) > 1:
				add(l.Line, RuleMapField, "`- %s` stands beside other lines in ## %s: delete one of them", listNone, list.heading)
			}
		}
	}
}

func checkMapIndex(m Map, add report) {
	if headingLine(Document{Headings: m.Headings}, 2, HeadingDecisionsSoFar) == 0 {
		return
	}
	closed := map[string]bool{}
	for _, t := range m.Tickets {
		closed[t.ID] = closed[t.ID] || t.Status == TicketClosed
	}
	listed := map[string]bool{}
	for _, l := range m.Decisions {
		item, ok := strings.CutPrefix(l.Text, bulletMark)
		if !ok || isNoneLine(l.Text) {
			continue
		}
		match := decisionLine.FindStringSubmatch(item)
		switch {
		case match == nil:
			add(l.Line, RuleMapIndex, "line %q has no ticket id and answer, the form is `- T1: answer`", l.Text)
		case !closed[match[1]]:
			add(l.Line, RuleMapIndex, "line %q names no closed ticket", l.Text)
		case listed[match[1]]:
			add(l.Line, RuleMapIndex, "closed ticket %s has two lines under ## %s", match[1], HeadingDecisionsSoFar)
		default:
			listed[match[1]] = true
		}
	}
	for _, t := range m.Tickets {
		if t.Status == TicketClosed && !listed[t.ID] {
			add(t.Line, RuleMapIndex, "closed ticket %s has no line under ## %s", t.ID, HeadingDecisionsSoFar)
		}
	}
}
