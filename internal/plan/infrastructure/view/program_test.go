package view_test

import (
	"encoding/json"
	"html"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

const (
	programFixture = "../../domain/testdata/program.md"
	programRuns    = 8
	programWaves   = 3
	injection      = "<script>alert(1)</script>"
)

var (
	runCard    = regexp.MustCompile(`(?s)<article id="run-([^"]+)" class="task run".*?</article>`)
	waveLane   = regexp.MustCompile(`class="wave"`)
	planData   = regexp.MustCompile(`(?s)<script type="application/json" id="plan-data">(.*?)</script>`)
	networkURL = regexp.MustCompile(`\b(?:src|href)="https?://`)
)

func programInput(t *testing.T) (domain.Program, domain.ViewInput) {
	t.Helper()
	raw, err := os.ReadFile(programFixture)
	if err != nil {
		t.Fatal(err)
	}
	prog, problems := domain.ParseProgram(string(raw))
	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	return prog, domain.ViewInput{Program: &prog}
}

func diagramsOf(t *testing.T, page string) []string {
	t.Helper()
	m := planData.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("the page has no plan-data script")
	}
	var data struct {
		Diagrams []string `json:"diagrams"`
	}
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		t.Fatalf("plan-data: %v", err)
	}
	return data.Diagrams
}

func TestProgramPage(t *testing.T) {
	prog, in := programInput(t)
	page := render(t, in)
	cards := map[string]string{}
	for _, m := range runCard.FindAllStringSubmatch(page, -1) {
		cards[m[1]] = m[0]
	}

	t.Run("a card holds the fields of each run", func(t *testing.T) {
		if len(cards) != programRuns || len(prog.Runs) != programRuns {
			t.Fatalf("cards = %d, runs = %d, want %d", len(cards), len(prog.Runs), programRuns)
		}
		for _, r := range prog.Runs {
			card := html.UnescapeString(cards[r.ID])
			want := append([]string{r.ID, r.Slug, r.Branch, r.DoneWhen}, r.Owned...)
			for _, step := range r.Scope {
				want = append(want, strings.ReplaceAll(step, "`", ""))
			}
			for _, w := range want {
				if !strings.Contains(strings.NewReplacer("<code>", "", "</code>", "").Replace(card), w) {
					t.Errorf("card %s lacks %q", r.ID, w)
				}
			}
		}
	})

	t.Run("a card links to each dependency", func(t *testing.T) {
		for _, r := range prog.Runs {
			for _, d := range r.DependsOn {
				if !strings.Contains(cards[r.ID], `href="#run-`+d+`"`) {
					t.Errorf("card %s lacks a link to %s", r.ID, d)
				}
			}
		}
		if missing := unresolvedLinks(page); len(missing) != 0 {
			t.Errorf("links without a target: %v", missing)
		}
	})

	t.Run("one lane for each wave", func(t *testing.T) {
		if got := len(waveLane.FindAllString(page, -1)); got != programWaves {
			t.Errorf("wave lanes = %d, want %d", got, programWaves)
		}
		first, second, third := strings.Index(page, "Wave 1"), strings.Index(page, "Wave 2"), strings.Index(page, "Wave 3")
		if first < 0 || first > second || second > third {
			t.Error("the lanes are not in wave order")
		}
	})

	t.Run("the diagram holds an edge for each dependency", func(t *testing.T) {
		diagrams := diagramsOf(t, page)
		if len(diagrams) != 1 || !strings.HasPrefix(diagrams[0], "flowchart LR") {
			t.Fatalf("diagrams = %q, want one flowchart LR", diagrams)
		}
		for _, r := range prog.Runs {
			if !strings.Contains(diagrams[0], r.ID+`["`+r.ID+" "+r.Slug+`"]`) {
				t.Errorf("no node for %s", r.ID)
			}
			for _, d := range r.DependsOn {
				if !strings.Contains(diagrams[0], d+" --> "+r.ID) {
					t.Errorf("no edge %s --> %s", d, r.ID)
				}
			}
		}
		if !strings.Contains(page, `data-diagram="0"`) {
			t.Error("no diagram node for view.js")
		}
	})

	t.Run("the measurement table and the decisions", func(t *testing.T) {
		text := html.UnescapeString(page)
		want := append(append([]string{}, prog.MeasurementHeader...), prog.Decisions...)
		for _, row := range prog.MeasurementRows {
			want = append(want, row...)
		}
		for _, w := range want {
			if !strings.Contains(text, w) {
				t.Errorf("the page lacks %q", w)
			}
		}
	})

	t.Run("a problem shows its rule and line", func(t *testing.T) {
		in.Problems = []domain.Problem{{Line: 12, Rule: domain.RuleRunField, Message: "run R1 has no Branch: line"}}
		row := problemRow.FindString(render(t, in))
		for _, w := range []string{"plan.md:12", string(domain.RuleRunField), "run R1 has no Branch"} {
			if !strings.Contains(row, w) {
				t.Errorf("problem row lacks %q: %s", w, row)
			}
		}
	})

	t.Run("the page loads nothing from the network", func(t *testing.T) {
		markup := scriptBlock.ReplaceAllString(page, "")
		if networkURL.MatchString(markup) {
			t.Error("a src or href loads from the network")
		}
		for _, m := range linkAttr.FindAllStringSubmatch(markup, -1) {
			if !strings.HasPrefix(m[1], "#") && !strings.HasPrefix(m[1], "data:") {
				t.Errorf("attribute %q points outside the page", m[0])
			}
		}
		if !strings.Contains(page, "font/woff2;base64,") || !strings.Contains(page, "mermaid") {
			t.Error("the page lacks the embedded fonts or the mermaid bundle")
		}
	})

	t.Run("plan text is escaped", func(t *testing.T) {
		evil := prog
		evil.Title = injection
		evil.Runs = append([]domain.ProgramRun(nil), prog.Runs...)
		evil.Runs[0].Scope = []string{injection}
		evil.Runs[0].DoneWhen = injection
		evil.Decisions = []string{injection}
		got := render(t, domain.ViewInput{Program: &evil})
		if strings.Contains(scriptBlock.ReplaceAllString(got, ""), injection) {
			t.Error("plan text reached the page as markup")
		}
	})
}
