package harness

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	guarddomain "github.com/jorge07/harness/internal/guard/domain"
	"github.com/jorge07/harness/internal/lint/infrastructure/packs"
	"github.com/jorge07/harness/internal/run/domain"
	shareddomain "github.com/jorge07/harness/internal/shared/domain"
)

const (
	skillFile        = "skills/harness/SKILL.md"
	grillFile        = "skills/harness/references/grill.md"
	explorerFile     = "agents/explorer.md"
	grillingSource   = "https://github.com/mattpocock/skills/blob/main/skills/productivity/grilling/SKILL.md"
	mitLicense       = "MIT"
	explorerRole     = "`explorer`"
	runPlanCommand   = "harness run plan"
	approveByCommand = "harness run approve --by"
	pluginsHeading   = "\n## Plugin events\n"
	hostsHeading     = "\n## Hosts\n"
	emDash           = "—"
)

var grillViews = []string{"Product", "QA", "Tech specs"}

func readDoc(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func section(t *testing.T, doc, heading string) string {
	t.Helper()
	_, rest, found := strings.Cut(doc, heading)
	if !found {
		t.Fatalf("no section %q", strings.TrimSpace(heading))
	}
	body, _, _ := strings.Cut(rest, nextHeading)
	return body
}

func TestHarnessSkillGrill(t *testing.T) {
	skill, grill, explorer := readDoc(t, skillFile), readDoc(t, grillFile), readDoc(t, explorerFile)

	explore := strings.Index(skill, explorerRole)
	submit := strings.Index(skill, runPlanCommand)
	if explore < 0 || submit < 0 || explore > submit {
		t.Errorf("%s: the explorer (at %d) must come before %s (at %d)", skillFile, explore, runPlanCommand, submit)
	}
	for _, want := range []string{grillFile[len("skills/harness/"):], shareddomain.ApprovalHeader, shareddomain.ApproveLabel, shareddomain.ChangePlanLabel} {
		if !strings.Contains(skill, want) {
			t.Errorf("%s does not name %q", skillFile, want)
		}
	}
	wants := []string{grillingSource, mitLicense, shareddomain.RecommendedSuffix[1:]}
	for _, tool := range shareddomain.QuestionTools {
		wants = append(wants, tool.Name)
	}
	for _, want := range wants {
		if !strings.Contains(grill, want) {
			t.Errorf("%s does not name %q", grillFile, want)
		}
	}
	last := -1
	for _, view := range grillViews {
		at := strings.Index(grill, "### "+view)
		if at <= last {
			t.Errorf("%s: the view %q is missing or out of order", grillFile, view)
		}
		last = at
	}
	for path, text := range map[string]string{skillFile: skill, grillFile: grill, explorerFile: explorer} {
		if strings.Contains(text, emDash) || strings.ContainsFunc(text, isEmoji) {
			t.Errorf("%s has an em dash or an emoji", path)
		}
	}
}

func isEmoji(r rune) bool {
	return r >= 0x1F300 && r <= 0x1FAFF || r >= 0x2600 && r <= 0x27BF
}

func TestDesignMatchesGuardRules(t *testing.T) {
	table := section(t, readDoc(t, designFile), pluginsHeading)
	for _, rule := range guarddomain.Rules() {
		if !strings.Contains(table, "| `"+rule.ID+"` |") {
			t.Errorf("%s has no guard row for %s", designFile, rule.ID)
		}
	}
}

func TestDesignMatchesQuestionTools(t *testing.T) {
	hosts := section(t, readDoc(t, designFile), hostsHeading)
	for host, tool := range shareddomain.QuestionTools {
		row := fmt.Sprintf("| `%s` | `%s` | %d | %d to %d |", host, tool.Name, tool.MaxQuestions, tool.MinOptions, tool.MaxOptions)
		if !strings.Contains(hosts, row) {
			t.Errorf("%s Hosts has no row %q", designFile, row)
		}
	}
	readme := readDoc(t, readmeFile)
	for _, want := range []string{"`" + shareddomain.ApprovalHeader + "` question", approveByCommand} {
		if !strings.Contains(readme, want) {
			t.Errorf("%s does not name %q", readmeFile, want)
		}
	}
}

const (
	designFile       = "DESIGN.md"
	loopHeading      = "\n## Loop\n"
	nextHeading      = "\n## "
	diagramOpen      = "```mermaid\nstateDiagram-v2\n"
	diagramClose     = "```"
	phaseNoneMarker  = "[*]"
	diagramBlockWant = 1
	lintHeading      = "\n### Lint packs\n"
	readmeFile       = "README.md"
	installHeading   = "\n## Install\n"
	subHeading       = "### "
)

var installPaths = []string{"Claude Code", "Codex", "Build from source"}

var (
	edgeLine     = regexp.MustCompile(`^\s*(\S+) --> (\S+): (\S+)(?: \[(.+)\])?$`)
	terminalLine = regexp.MustCompile(`^\s*(\S+) --> \[\*\]$`)
	ruleRow      = regexp.MustCompile("^\\| `([a-z-]+/[a-z-]+)` \\|")
	lintEnd      = regexp.MustCompile(`\n#{2,3} `)
)

type edge struct {
	from, event, guard, to string
}

func TestDesignDiagramMatchesTable(t *testing.T) {
	raw, err := os.ReadFile(designFile)
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
	for _, row := range domain.Transitions() {
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
	raw, err := os.ReadFile(designFile)
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

func TestReadmeInstallOrder(t *testing.T) {
	raw, err := os.ReadFile(readmeFile)
	if err != nil {
		t.Fatal(err)
	}
	_, rest, found := strings.Cut(string(raw), installHeading)
	if !found {
		t.Fatalf("%s has no Install section", readmeFile)
	}
	section, _, _ := strings.Cut(rest, nextHeading)
	var got []string
	for _, line := range strings.Split(section, "\n") {
		if title, ok := strings.CutPrefix(line, subHeading); ok {
			got = append(got, title)
		}
	}
	if !slices.Equal(got, installPaths) {
		t.Errorf("Install subsections = %q, want %q", got, installPaths)
	}
}

func phaseName(label string) string {
	if label == phaseNoneMarker {
		return string(domain.PhaseNone)
	}
	return label
}
