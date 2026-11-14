package domain_test

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

const (
	mapFixture = "testdata/map.md"
	minimalMap = "# Map: Small\n\n## Destination\n\nText.\n\n## Decisions so far\n\n- none\n\n## Not yet specified\n\n- none\n\n## Out of scope\n\n- none\n\n## Tickets\n"
)

func readMap(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(mapFixture)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func checkMapText(text string) []domain.Problem {
	m, problems := domain.ParseMap(text)
	problems = append(problems, domain.CheckMap(m)...)
	domain.SortProblems(problems)
	return problems
}

func exactLine(t *testing.T, text, line string) int {
	t.Helper()
	for i, l := range strings.Split(text, "\n") {
		if l == line {
			return i + 1
		}
	}
	t.Fatalf("text has no line %q", line)
	return 0
}

func TestParseMap(t *testing.T) {
	m, problems := domain.ParseMap(readMap(t))
	if len(problems) != 0 {
		t.Fatalf("fixture has parse problems: %v", problems)
	}
	if m.Title != "Map: Invoice export" || len(m.Tickets) != 7 || len(m.Decisions) != 2 {
		t.Fatalf("title = %q, tickets = %d, decisions = %d", m.Title, len(m.Tickets), len(m.Decisions))
	}
	t2, t5, t6 := m.Tickets[1], m.Tickets[4], m.Tickets[5]
	if t2.ID != "T2" || t2.Slug != "run-mode" || t2.Type != domain.TicketResearch || t2.Status != domain.TicketClosed || !strings.HasPrefix(t2.Answer, "As a job") {
		t.Errorf("T2 = %+v", t2)
	}
	if t5.Claim != "ana" || t5.Question == "" || len(t5.FieldLines) != 5 {
		t.Errorf("T5 = %+v", t5)
	}
	if !slices.Equal(t6.BlockedBy, []string{"T1", "T3", "T5"}) {
		t.Errorf("T6 blockers = %v", t6.BlockedBy)
	}
	if len(domain.MapFields) != 6 || len(domain.TicketTypes) != 4 || len(domain.TicketStatuses) != 2 {
		t.Errorf("lists = %v, %v, %v", domain.MapFields, domain.TicketTypes, domain.TicketStatuses)
	}
}

func TestMapUnclosedFenceInRefusedSection(t *testing.T) {
	text := "# Map: x\n\n## Destination\n\nA place.\n\n## Not yet specified\n\n```md\n- hidden item\n"

	_, problems := domain.ParseMap(text)

	var fenceLines []string
	for _, p := range problems {
		if strings.Contains(p.Message, "code fence") {
			fenceLines = append(fenceLines, p.String())
		}
	}
	if len(fenceLines) != 1 || !strings.Contains(fenceLines[0], "a code fence is not allowed under ## Not yet specified") || !strings.HasPrefix(fenceLines[0], "plan.md:9:") {
		t.Fatalf("fence problems = %v, want only the not allowed line at line 9", fenceLines)
	}
}

func TestMapCheck(t *testing.T) {
	valid := readMap(t)
	if got := checkMapText(valid); len(got) != 0 {
		t.Fatalf("fixture has problems: %v", got)
	}

	const (
		t2Index = "- T2: the export runs as a job and not as a request.\n"
		t2Fix   = "- Blocked by: T1\n- Status: closed"
		t7Block = "- Blocked by: T2\n"
		t7Ask   = "Who files the access request for the bucket?"
	)
	longSentence := strings.TrimSpace(strings.Repeat("word ", 26)) + "."
	tests := []struct {
		name  string
		text  string
		edits []edit
		rule  domain.RuleID
		line  func(text string) int
	}{
		{"no notes", minimalMap, nil, "", nil},
		{"subheading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### Background\n"}}, "", nil},
		{"unknown heading after the tickets", valid, []edit{{"### T7 bucket-access", "## Extra\n\n### T7 bucket-access"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "## Extra") }},
		{"unknown heading before the last ticket", valid, []edit{{"### T7 bucket-access", "## Later\n\n### T7 bucket-access"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "## Later") }},
		{"open ticket under out of scope", valid, []edit{{"- The retention of old exports.\n", "- The retention of old exports.\n\n### T8 stray-ticket\n\n- Type: work\n- Blocked by: none\n- Status: open\n- Question: Who owns the stray ticket?\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8 stray-ticket") }},
		{"open ticket under notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T8 stray-ticket\n\n- Type: work\n- Blocked by: none\n- Status: open\n- Question: Who owns the stray ticket?\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8 stray-ticket") }},
		{"open ticket above destination", valid, []edit{{"## Destination\n", "### T8 stray-ticket\n\n- Type: work\n- Blocked by: none\n- Status: open\n- Question: Who owns the stray ticket?\n\n## Destination\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8 stray-ticket") }},
		{"malformed T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T2 and T3 background\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T2 and T3 background") }},
		{"capital slug T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T8 Hidden-one\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8 Hidden-one") }},
		{"underscore slug T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T8 hidden_one\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8 hidden_one") }},
		{"level four T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n#### T8 x\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "#### T8 x") }},
		{"indented T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n   ### T8 x\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "   ### T8 x") }},
		{"bare T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T8\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8") }},
		{"T word heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### Tools and T8x\n"}}, "", nil},
		{"hyphen T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T8-hidden\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8-hidden") }},
		{"dot T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T8.hidden\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8.hidden") }},
		{"T id with a letter suffix in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T8x\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8x") }},
		{"underscore T heading in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### T8_hidden\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T8_hidden") }},
		{"T word with no digit in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n### Tx8 notes\n"}}, "", nil},
		{"fence under not yet specified", valid, []edit{{"## Not yet specified\n", "## Not yet specified\n\n```md\n- hidden item\n```\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "```md") }},
		{"fence under out of scope", valid, []edit{{"## Out of scope\n", "## Out of scope\n\n```md\n- hidden item\n```\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "```md") }},
		{"fence under decisions so far", valid, []edit{{"## Decisions so far\n", "## Decisions so far\n\n```md\n- hidden item\n```\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "```md") }},
		{"fence in the destination", valid, []edit{{"## Destination\n", "## Destination\n\n```sh\n### T0 shown\n```\n"}}, "", nil},
		{"missing question", valid, []edit{{"- Question: " + t7Ask + "\n", ""}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "### T7") }},
		{"no destination", valid, []edit{{"## Destination\n", ""}}, domain.RuleHeadingMissing, func(string) int { return 1 }},
		{"no decisions", valid, []edit{{"## Decisions so far\n", ""}}, domain.RuleHeadingMissing, func(string) int { return 1 }},
		{"no tickets", strings.TrimSuffix(minimalMap, "\n## Tickets\n"), nil, domain.RuleHeadingMissing, func(string) int { return 1 }},
		{"headings out of order", valid, []edit{{"## Not yet specified", "## @@"}, {"## Out of scope", "## Not yet specified"}, {"## @@", "## Out of scope"}},
			domain.RuleHeadingMissing, func(s string) int { return lineOf(t, s, "## Not yet specified") }},
		{"bad ticket heading", valid, []edit{{"### T7 bucket-access", "### Ticket seven"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "### Ticket seven") }},
		{"duplicate id", valid, []edit{{"### T7 bucket-access", "### T6 bucket-access"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "### T6 bucket-access") }},
		{"missing field", valid, []edit{{"- Type: work\n", ""}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "### T7") }},
		{"unknown field", valid, []edit{{"- Type: work\n", "- Type: work\n- Owner: ana\n"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "- Owner: ana") }},
		{"bullet without a colon", valid, []edit{{"- Type: work\n", "- Type: work\n- Owner ana\n"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "- Owner ana") }},
		{"repeated field", valid, []edit{{t7Block, t7Block + t7Block}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "### T7") + 4 }},
		{"field before any ticket", valid, []edit{{"## Tickets\n", "## Tickets\n\n- Type: work\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "- Type: work") }},
		{"bad type", valid, []edit{{"- Type: work", "- Type: task"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "Type: task") }},
		{"bad status", valid, []edit{{t7Block + "- Status: open", t7Block + "- Status: pending"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "Status: pending") }},
		{"bad blocked form", valid, []edit{{t7Block, "- Blocked by: T2 or T3\n"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "T2 or T3") }},
		{"blank blocked", valid, []edit{{t7Block, "- Blocked by:\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "- Blocked by:") }},
		{"closed without answer", valid, []edit{{"- Answer: As a job, because a large account takes longer than a request limit.\n", ""}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "### T2") }},
		{"open with answer", valid, []edit{{"- Question: " + t7Ask + "\n", "- Question: " + t7Ask + "\n- Answer: Ana files it.\n"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "Answer: Ana") }},
		{"closed with claim", valid, []edit{{t2Fix, "- Blocked by: T1\n- Status: closed\n- Claimed by: ana"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "- Claimed by: ana") }},
		{"blocked by after status", valid, []edit{{t7Block + "- Status: open", "- Status: open\n" + strings.TrimSuffix(t7Block, "\n")}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "- Blocked by: T2") }},
		{"claim before status", valid, []edit{{"- Status: open\n- Claimed by: ana", "- Claimed by: ana\n- Status: open"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "- Claimed by: ana") + 1 }},
		{"answer before question", valid, []edit{{"- Question: Which file format does the export use?\n- Answer: A CSV file with one row for each invoice line.", "- Answer: A CSV file with one row for each invoice line.\n- Question: Which file format does the export use?"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "- Question: Which file format") }},
		{"indented bullet in a ticket", valid, []edit{{"- Claimed by: ana", "  - Claimed by: ana"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "  - Claimed by: ana") }},
		{"star bullet in a ticket", valid, []edit{{"- Claimed by: ana", "* Claimed by: ana"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "* Claimed by: ana") }},
		{"plain line in a ticket", valid, []edit{{"- Claimed by: ana", "Claimed by: ana"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "Claimed by: ana") }},
		{"empty claim", valid, []edit{{"- Claimed by: ana", "- Claimed by:"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "- Claimed by:") }},
		{"claim none", valid, []edit{{"- Claimed by: ana", "- Claimed by: none"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "- Claimed by: none") }},
		{"claim None with full stop", valid, []edit{{"- Claimed by: ana", "- Claimed by: None."}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "- Claimed by: None.") }},
		{"unclosed fence in a ticket", valid, []edit{{"- Question: How long does a download link stay valid?\n", "- Question: How long does a download link stay valid?\n```sh\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "```sh") }},
		{"prose before the first ticket", valid, []edit{{"## Tickets\n", "## Tickets\n\nSome prose.\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "Some prose.") }},
		{"prose after a malformed ticket heading", valid, []edit{{"## Tickets\n", "## Tickets\n\n### T0 Bad Slug\n\nSome prose.\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "### T0 Bad Slug") }},
		{"closed fence hides a ticket", valid, []edit{{"## Tickets\n", "## Tickets\n\n```md\n### T0 hidden\n```\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "```md") }},
		{"closed fence in a ticket", valid, []edit{{"- Question: How long does a download link stay valid?\n", "- Question: How long does a download link stay valid?\n```sh\nx\n```\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "```sh") }},
		{"fence in the notes", valid, []edit{{"## Notes\n", "## Notes\n\n```sh\n### T0 shown\n```\n"}}, "", nil},
		{"question mark in a code span", valid, []edit{{"- Question: " + t7Ask, "- Question: Does `a?b` work here?"}}, "", nil},
		{"none with a capital in a list", strings.Replace(minimalMap, "## Decisions so far\n\n- none\n", "## Decisions so far\n\n- None.\n", 1), nil, "", nil},
		{"none with a full stop in a list", strings.Replace(minimalMap, "## Not yet specified\n\n- none\n", "## Not yet specified\n\n- none.\n", 1), nil, "", nil},
		{"title without the map prefix", valid, []edit{{"# Map: Invoice export", "# Invoice export"}}, domain.RuleMapField, func(string) int { return 1 }},
		{"title with no text", valid, []edit{{"# Map: Invoice export", "# Map:"}}, domain.RuleMapField, func(string) int { return 1 }},
		{"no title line", valid, []edit{{"# Map: Invoice export\n", ""}}, domain.RuleMapField, func(string) int { return 1 }},
		{"destination without text", valid, []edit{{"Customers download a monthly invoice export as a CSV file.\nThe map holds the decisions before the plan.\n", ""}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "## Destination") }},
		{"closed ticket blocked by an open ticket", valid, []edit{{t2Fix, "- Blocked by: T1 and T5\n- Status: closed"}}, domain.RuleMapBlocked, func(s string) int { return lineOf(t, s, "- Blocked by: T1 and T5") }},
		{"blocker is missing", valid, []edit{{t7Block, "- Blocked by: T9\n"}}, domain.RuleMapBlocked, func(s string) int { return lineOf(t, s, "Blocked by: T9") }},
		{"ticket blocks itself", valid, []edit{{t7Block, "- Blocked by: T7\n"}}, domain.RuleMapBlocked, func(s string) int { return lineOf(t, s, "Blocked by: T7") }},
		{"blockers make a cycle", valid, []edit{{"- Blocked by: none\n- Status: closed", "- Blocked by: T2\n- Status: closed"}}, domain.RuleMapBlocked, func(s string) int { return lineOf(t, s, "- Blocked by: T2") }},
		{"question without a mark", valid, []edit{{t7Ask, "Who files the access request for the bucket."}}, domain.RuleMapQuestion, func(s string) int { return lineOf(t, s, "bucket.") }},
		{"two questions", valid, []edit{{t7Ask, "Who files the request? Who signs it?"}}, domain.RuleMapQuestion, func(s string) int { return lineOf(t, s, "Who signs it?") }},
		{"closed ticket not indexed", valid, []edit{{t2Index, ""}}, domain.RuleMapIndex, func(s string) int { return lineOf(t, s, "### T2") }},
		{"index names an open ticket", valid, []edit{{t2Index, t2Index + "- T7: someone files it.\n"}}, domain.RuleMapIndex, func(s string) int { return lineOf(t, s, "- T7:") }},
		{"index line without an id", valid, []edit{{t2Index, t2Index + "- the export is nice.\n"}}, domain.RuleMapIndex, func(s string) int { return lineOf(t, s, "- the export is nice.") }},
		{"index names a ticket twice", valid, []edit{{t2Index, "- T1: the export again.\n" + t2Index}}, domain.RuleMapIndex, func(s string) int { return lineOf(t, s, "- T1: the export again.") }},
		{"decisions stray line", valid, []edit{{t2Index, t2Index + "A stray line.\n"}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "A stray line.") }},
		{"list line without a bullet", valid, []edit{{"- The retention of old exports.", "The retention of old exports."}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "The retention of old") }},
		{"none beside a bullet", valid, []edit{{"- The retention of old exports.\n", "- none\n- The retention of old exports.\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "- none") }},
		{"none with two spaces beside a bullet", valid, []edit{{"- The retention of old exports.\n", "-  none\n- The retention of old exports.\n"}}, domain.RuleMapField, func(s string) int { return exactLine(t, s, "-  none") }},
		{"empty list", valid, []edit{{"- none\n", ""}}, domain.RuleMapField, func(s string) int { return lineOf(t, s, "## Out of scope") }},
		{"long sentence", valid, []edit{{"A ticket ends with an answer.", longSentence}}, domain.RuleSentenceLength, func(s string) int { return lineOf(t, s, "word word") }},
		{"em dash", valid, []edit{{"A ticket ends", "A ticket — ends"}}, domain.RuleEmDash, func(s string) int { return lineOf(t, s, "—") }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.text
			if len(tc.edits) > 0 {
				text = mutate(t, text, tc.edits...)
			}
			got := checkMapText(text)
			if tc.rule == "" {
				if len(got) != 0 {
					t.Fatalf("problems = %v, want none", got)
				}
				return
			}
			want := fmt.Sprintf("plan.md:%d: %s:", tc.line(text), tc.rule)
			if len(got) != 1 || !strings.HasPrefix(got[0].String(), want) {
				t.Fatalf("problems = %v, want one %q", got, want)
			}
		})
	}
}

func TestMapCheckSortsProblems(t *testing.T) {
	text := mutate(t, readMap(t), edit{"- Type: work", "- Type: task"}, edit{"- Blocked by: T2\n", "- Blocked by: T9\n"}, edit{"## Destination\n", ""})
	got := checkMapText(text)
	if len(got) != 3 || got[0].Rule != domain.RuleHeadingMissing || got[1].Rule != domain.RuleMapField || got[2].Rule != domain.RuleMapBlocked {
		t.Fatalf("problems = %v", got)
	}
}

func TestMapIndexMessages(t *testing.T) {
	valid := readMap(t)
	t2Index := "- T2: the export runs as a job and not as a request.\n"
	tests := []struct {
		name string
		edit edit
		want string
	}{
		{"no ticket id", edit{t2Index, t2Index + "- the export is nice.\n"}, "- T1: answer"},
		{"wrong ticket id", edit{t2Index, t2Index + "- T7: someone files it.\n"}, "names no closed ticket"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := checkMapText(mutate(t, valid, tc.edit))
			if len(got) != 1 || got[0].Rule != domain.RuleMapIndex || !strings.Contains(got[0].Message, tc.want) {
				t.Fatalf("problems = %v, want one %s with %q", got, domain.RuleMapIndex, tc.want)
			}
		})
	}
}

func ticket(id, status, claim string, blocked ...string) domain.Ticket {
	return domain.Ticket{ID: id, Status: status, Claim: claim, BlockedBy: blocked}
}

func TestMapFrontier(t *testing.T) {
	open, closed := domain.TicketOpen, domain.TicketClosed
	tests := []struct {
		name    string
		tickets []domain.Ticket
		want    []string
	}{
		{"open and free", []domain.Ticket{ticket("T1", open, "")}, []string{"T1"}},
		{"closed", []domain.Ticket{ticket("T1", closed, "")}, nil},
		{"claimed", []domain.Ticket{ticket("T1", open, "ana")}, nil},
		{"blocked by an open ticket", []domain.Ticket{ticket("T1", open, ""), ticket("T2", open, "", "T1")}, []string{"T1"}},
		{"blocked by a closed ticket", []domain.Ticket{ticket("T1", closed, ""), ticket("T2", open, "", "T1")}, []string{"T2"}},
		{"blocked by a missing ticket", []domain.Ticket{ticket("T2", open, "", "T9")}, nil},
		{"blocked by two tickets, one open", []domain.Ticket{ticket("T1", closed, ""), ticket("T2", open, ""), ticket("T3", open, "ana"), ticket("T4", open, "", "T1", "T2")}, []string{"T2"}},
		{"file order", []domain.Ticket{ticket("T3", open, ""), ticket("T1", open, "")}, []string{"T3", "T1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			for _, f := range (domain.Map{Tickets: tc.tickets}).Frontier() {
				got = append(got, f.ID)
			}
			if !slices.Equal(got, tc.want) {
				t.Errorf("frontier = %v, want %v", got, tc.want)
			}
		})
	}
	t.Run("fixture", func(t *testing.T) {
		m, _ := domain.ParseMap(readMap(t))
		var got []string
		for _, f := range m.Frontier() {
			got = append(got, f.ID)
		}
		if want := []string{"T3", "T7"}; !slices.Equal(got, want) {
			t.Errorf("frontier = %v, want %v", got, want)
		}
	})
}

func TestMapClear(t *testing.T) {
	none := []domain.MapLine{{Text: "- none", Line: 1}}
	bullets := []domain.MapLine{{Text: "- a question", Line: 1}}
	tests := []struct {
		name string
		m    domain.Map
		want bool
	}{
		{"closed and nothing unspecified", domain.Map{Tickets: []domain.Ticket{ticket("T1", domain.TicketClosed, "")}, Unspecified: none}, true},
		{"no ticket at all", domain.Map{Unspecified: none}, true},
		{"an open ticket", domain.Map{Tickets: []domain.Ticket{ticket("T1", domain.TicketOpen, "")}, Unspecified: none}, false},
		{"none with two spaces", domain.Map{Unspecified: []domain.MapLine{{Text: "-  none", Line: 1}}}, true},
		{"none with a full stop", domain.Map{Tickets: []domain.Ticket{ticket("T1", domain.TicketClosed, "")}, Unspecified: []domain.MapLine{{Text: "- none.", Line: 1}}}, true},
		{"None with a capital", domain.Map{Unspecified: []domain.MapLine{{Text: "- None", Line: 1}}}, true},
		{"NONE with a full stop", domain.Map{Unspecified: []domain.MapLine{{Text: "- NONE.", Line: 1}}}, true},
		{"something unspecified", domain.Map{Unspecified: bullets}, false},
		{"empty section", domain.Map{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.m.Clear(); got != tc.want {
				t.Errorf("Clear = %v, want %v", got, tc.want)
			}
		})
	}
	if m, _ := domain.ParseMap(readMap(t)); m.Clear() {
		t.Error("the fixture has open tickets and is not clear")
	}
}

func TestIsMap(t *testing.T) {
	valid, err := os.ReadFile("testdata/valid.md")
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name        string
		text        string
		wantMap     bool
		wantProgram bool
	}{
		{"plain map", readMap(t), true, false},
		{"single-run plan", string(valid), false, false},
		{"multi-run plan", readProgram(t), false, true},
		{"both headings", "# T\n\n## Destination\n\nText.\n\n## Runs\n", true, true},
		{"destination line inside a fence", "# T\n\n```md\n## Destination\n```\n", false, false},
		{"destination line after a fence", "# T\n\n```md\ntext\n```\n\n## Destination\n", true, false},
		{"destination heading with text", "# T\n\n## Destination and more\n", false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := domain.IsMap(tc.text); got != tc.wantMap {
				t.Errorf("IsMap = %v, want %v", got, tc.wantMap)
			}
			if got := domain.IsProgram(tc.text); got != tc.wantProgram {
				t.Errorf("IsProgram = %v, want %v", got, tc.wantProgram)
			}
		})
	}
}
