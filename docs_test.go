package harness

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jorge07/harness/internal/run/domain"
)

const (
	designFile       = "DESIGN.md"
	loopHeading      = "\n## Loop\n"
	nextHeading      = "\n## "
	diagramOpen      = "```mermaid\nstateDiagram-v2\n"
	diagramClose     = "```"
	phaseNoneMarker  = "[*]"
	diagramBlockWant = 1
)

var (
	edgeLine     = regexp.MustCompile(`^\s*(\S+) --> (\S+): (\S+)(?: \[(.+)\])?$`)
	terminalLine = regexp.MustCompile(`^\s*(\S+) --> \[\*\]$`)
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

func phaseName(label string) string {
	if label == phaseNoneMarker {
		return string(domain.PhaseNone)
	}
	return label
}
