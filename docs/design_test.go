package docs_test

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	guarddomain "github.com/90p-labs/harness/internal/guard/domain"
	"github.com/90p-labs/harness/internal/lint/infrastructure/packs"
	plandomain "github.com/90p-labs/harness/internal/plan/domain"
	rundomain "github.com/90p-labs/harness/internal/run/domain"
	shareddomain "github.com/90p-labs/harness/internal/shared/domain"
	"github.com/90p-labs/harness/internal/testsupport/repository"
)

const (
	approveByCommand = "harness run approve --by"
	pluginsHeading   = "\n## Plugin events\n"
	hostsHeading     = "\n## Hosts\n"
	designFile       = "DESIGN.md"
	loopHeading      = "\n## Loop\n"
	nextHeading      = "\n## "
	diagramOpen      = "```mermaid\nstateDiagram-v2\n"
	diagramClose     = "```"
	phaseNoneMarker  = "[*]"
	diagramBlockWant = 1
	lintHeading      = "\n### Lint packs\n"
	readmeFile       = "README.md"
	configDocFile    = "docs/configuration.md"
	runsDocFile      = "docs/runs.md"
	multiRunHeading  = "\n## Multi-run plan\n"
	multiRunDesign   = "\n### Multi-run plan\n"
	mapHeading       = "\n## Map\n"
	mapDesign        = "\n### Map\n"
	markdownFence    = "```markdown\n"
	beforePayload    = "`run.transition.before` payload `to`"
)

var (
	edgeLine     = regexp.MustCompile(`^\s*(\S+) --> (\S+): (\S+)(?: \[(.+)\])?$`)
	terminalLine = regexp.MustCompile(`^\s*(\S+) --> \[\*\]$`)
	ruleRow      = regexp.MustCompile("^\\| `([a-z-]+/[a-z-]+)` \\|")
	lintEnd      = regexp.MustCompile(`\n#{2,3} `)
)

func TestDesignMatchesGuardRules(t *testing.T) {
	table := repository.Section(t, repository.Read(t, designFile), pluginsHeading)
	for _, rule := range guarddomain.Rules() {
		if !strings.Contains(table, "| `"+rule.ID+"` |") {
			t.Errorf("%s has no guard row for %s", designFile, rule.ID)
		}
	}
}

func TestDesignMatchesQuestionTools(t *testing.T) {
	hosts := repository.Section(t, repository.Read(t, designFile), hostsHeading)
	for host, tool := range shareddomain.QuestionTools {
		row := fmt.Sprintf("| `%s` | `%s` | %d | %d to %d |", host, tool.Name, tool.MaxQuestions, tool.MinOptions, tool.MaxOptions)
		if !strings.Contains(hosts, row) {
			t.Errorf("%s Hosts has no row %q", designFile, row)
		}
	}
	readme := repository.Read(t, readmeFile)
	for _, want := range []string{"`" + shareddomain.ApprovalHeader + "` question", approveByCommand} {
		if !strings.Contains(readme, want) {
			t.Errorf("%s does not name %q", readmeFile, want)
		}
	}
}

type edge struct {
	from, event, guard, to string
}

func TestDesignDiagramMatchesTable(t *testing.T) {
	raw, err := os.ReadFile(repository.Path(t, designFile))
	if err != nil {
		t.Fatal(err)
	}
	section := loopSection(t, string(raw))
	if got := strings.Count(section, diagramOpen); got != diagramBlockWant {
		t.Fatalf("Loop section has %d stateDiagram-v2 blocks, want %d", got, diagramBlockWant)
	}

	drawn, terminals := parseDiagram(section)
	want := map[edge]bool{}
	rowsFrom := map[string]bool{}
	for _, row := range rundomain.Transitions() {
		want[edge{string(row.From), string(row.Event), row.Guard.Name, string(row.To)}] = true
		rowsFrom[string(row.From)] = true
	}

	for e := range drawn {
		if !want[e] {
			t.Errorf("diagram edge not in table: %+v", e)
		}
	}
	for e := range want {
		if !drawn[e] {
			t.Errorf("table row not in diagram: %+v", e)
		}
	}
	for _, phase := range terminals {
		if rowsFrom[phase] {
			t.Errorf("terminal phase %q has an outgoing row", phase)
		}
	}
}

func loopSection(t *testing.T, doc string) string {
	t.Helper()
	_, rest, found := strings.Cut(doc, loopHeading)
	if !found {
		t.Fatalf("%s has no Loop section", designFile)
	}
	section, _, _ := strings.Cut(rest, nextHeading)
	return section
}

func parseDiagram(section string) (map[edge]bool, []string) {
	_, rest, _ := strings.Cut(section, diagramOpen)
	body, _, _ := strings.Cut(rest, diagramClose)
	drawn := map[edge]bool{}
	var terminals []string
	for _, line := range strings.Split(body, "\n") {
		if m := terminalLine.FindStringSubmatch(line); m != nil {
			terminals = append(terminals, m[1])
		} else if m := edgeLine.FindStringSubmatch(line); m != nil {
			drawn[edge{phaseName(m[1]), m[3], m[4], m[2]}] = true
		}
	}
	return drawn, terminals
}

func TestDesignLintRulesMatchCatalogue(t *testing.T) {
	raw, err := os.ReadFile(repository.Path(t, designFile))
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(raw), lintHeading)
	if !found {
		t.Fatalf("%s has no Lint packs section", designFile)
	}
	section := rest
	if next := lintEnd.FindStringIndex(rest); next != nil {
		section = rest[:next[0]]
	}

	documented := map[string]bool{}
	for _, line := range strings.Split(section, "\n") {
		if m := ruleRow.FindStringSubmatch(line); m != nil {
			documented[m[1]] = true
		}
	}

	all, err := packs.All()
	if err != nil {
		t.Fatal(err)
	}
	shipped := map[string]bool{}
	for _, pack := range all {
		for _, rule := range pack.Rules {
			shipped[rule.ID] = true
			if !documented[rule.ID] {
				t.Errorf("rule %s is not in the DESIGN.md tables", rule.ID)
			}
		}
	}
	for id := range documented {
		if !shipped[id] {
			t.Errorf("DESIGN.md rule %s is not in the catalogue", id)
		}
	}
}

func TestFilterDocs(t *testing.T) {
	names := append(slices.Clone(shareddomain.Transitions), shareddomain.Phases...)
	for _, path := range []string{configDocFile, designFile} {
		doc := repository.Read(t, path)
		for _, name := range names {
			if !strings.Contains(doc, "`"+name+"`") {
				t.Errorf("%s does not name `%s`", path, name)
			}
		}
		if !strings.Contains(doc, beforePayload) {
			t.Errorf("%s does not mention %s", path, beforePayload)
		}
	}
}

func phaseName(label string) string {
	if label == phaseNoneMarker {
		return string(rundomain.PhaseNone)
	}
	return label
}

func TestMultiRunPlanExample(t *testing.T) {
	_, rest, found := strings.Cut(repository.Read(t, runsDocFile), multiRunHeading)
	if !found {
		t.Fatalf("%s has no Multi-run plan section", runsDocFile)
	}
	_, rest, found = strings.Cut(rest, markdownFence)
	if !found {
		t.Fatalf("%s has no markdown example", runsDocFile)
	}
	example, _, found := strings.Cut(rest, diagramClose)
	if !found {
		t.Fatalf("%s has an unclosed markdown example", runsDocFile)
	}
	if !plandomain.IsProgram(example) {
		t.Fatal("the example is not a multi-run plan")
	}
	prog, problems := plandomain.ParseProgram(example)
	problems = append(problems, plandomain.CheckProgram(prog)...)
	if len(problems) != 0 {
		t.Errorf("the example has problems: %v", problems)
	}
	if !strings.Contains(repository.Read(t, designFile), multiRunDesign) {
		t.Errorf("%s has no Multi-run plan heading", designFile)
	}
}

func TestMapExample(t *testing.T) {
	_, rest, found := strings.Cut(repository.Read(t, runsDocFile), mapHeading)
	if !found {
		t.Fatalf("%s has no Map section", runsDocFile)
	}
	head, rest, found := strings.Cut(rest, markdownFence)
	if !found || strings.Contains(head, nextHeading) {
		t.Fatalf("%s has no map example", runsDocFile)
	}
	example, rest, found := strings.Cut(rest, diagramClose)
	if !found {
		t.Fatalf("%s has an unclosed map example", runsDocFile)
	}
	section, _, _ := strings.Cut(rest, nextHeading)
	section = head + section
	if !plandomain.IsMap(example) {
		t.Fatal("the example is not a map")
	}
	m, problems := plandomain.ParseMap(example)
	problems = append(problems, plandomain.CheckMap(m)...)
	if len(problems) != 0 {
		t.Errorf("the example has problems: %v", problems)
	}
	free := m.Frontier()
	if len(free) == 0 {
		t.Fatal("the example has no free ticket")
	}
	for _, ticket := range free {
		want := fmt.Sprintf("frontier: %s %s (%s)", ticket.ID, ticket.Slug, ticket.Type)
		if !strings.Contains(section, want) {
			t.Errorf("%s does not show the output %q", runsDocFile, want)
		}
	}
	for _, want := range []string{"harness plan check --file", "ADR 0026", "docs/programs/run-program-layers/", "`harness run plan`", "`harness plan view`"} {
		if !strings.Contains(section, want) {
			t.Errorf("the Map section of %s does not say %q", runsDocFile, want)
		}
	}
}

func TestMapDesign(t *testing.T) {
	design := repository.Read(t, designFile)
	_, rest, found := strings.Cut(design, mapDesign)
	if !found {
		t.Fatalf("%s has no Map heading", designFile)
	}
	section, _, _ := strings.Cut(rest, "\n## ")
	wants := []string{"frontier: T<n> slug (type)", "frontier: none", "map: clear", "plan-kind", "`harness plan view`", "`harness run plan`"}
	types, statuses, fields := plandomain.TicketTypes, plandomain.TicketStatuses, plandomain.MapFields
	wants = append(wants,
		"Type is one of "+strings.Join(types[:len(types)-1], ", ")+" and "+types[len(types)-1]+".",
		"Status is one of "+statuses[0]+" and "+statuses[1]+".",
		"The fields are "+strings.Join(fields[:3], ", ")+", optional "+fields[3]+", "+fields[4]+", and "+fields[5]+" for a closed ticket.",
	)
	for _, rule := range []plandomain.RuleID{plandomain.RuleMapField, plandomain.RuleMapBlocked, plandomain.RuleMapQuestion, plandomain.RuleMapIndex} {
		wants = append(wants, "The rule `"+string(rule)+"`")
	}
	for _, want := range wants {
		if !strings.Contains(section, want) {
			t.Errorf("the Map section of %s does not say %q", designFile, want)
		}
	}
	if strings.Index(design, mapDesign) > strings.Index(design, "\n## Gates and lint packs\n") {
		t.Errorf("the Map section of %s is not under Plan commands", designFile)
	}
}

func TestHandoffDocsListEveryProblem(t *testing.T) {
	wants := []string{
		"One refusal lists every problem.",
		"Each section has at least one `- ` line.",
	}
	for _, file := range []string{designFile, configDocFile} {
		doc := repository.Read(t, file)
		for _, want := range wants {
			if !strings.Contains(doc, want) {
				t.Errorf("%s does not say %q", file, want)
			}
		}
	}
}

func TestHandoffDocs(t *testing.T) {
	wants := []string{
		"handoff.md",
		fmt.Sprintf("%d byte", rundomain.HandoffLimit),
		"## Next",
		"## Open findings",
		"## Unreviewed commits",
		"## Decisions",
		"handoff_step",
		"`handoff.md` is a checkpoint",
		"The `handoff:` line ends with `then continue`",
		"No host resumes a run by itself",
	}
	for _, file := range []string{designFile, configDocFile} {
		doc := repository.Read(t, file)
		if strings.Contains(doc, "no pane restart") {
			t.Errorf("%s still says %q", file, "no pane restart")
		}
		for _, want := range wants {
			if !strings.Contains(doc, want) {
				t.Errorf("%s does not name %q", file, want)
			}
		}
	}
}
