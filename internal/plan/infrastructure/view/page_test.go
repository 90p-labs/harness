package view_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
	"github.com/90p-labs/harness/internal/plan/infrastructure/view"
)

const (
	pongFile     = "internal/ping/domain/pong.go"
	legacyFile   = "internal/ping/domain/legacy.go"
	moduleFile   = "internal/ping/module.go"
	listSketch   = "ui/list.excalidraw"
	pongSubject  = "feat: add the pong domain"
	querySubject = "feat: add the ping query"
	brokenSVG    = `<svg viewBox="0 0 10 10"><text>list sketch</text></svg>`
)

const viewPlan = "# Plan: add the ping query\n" +
	"\n## Goal\n\nThe service answers a ping with the word pong.\n" +
	"\n## Acceptance criteria\n\n" +
	"| # | Criterion | Proof |\n|---|---|---|\n" +
	"| 1 | The domain gives the word pong. | `go test ./internal/ping/domain -run TestPong -count=1` |\n" +
	"| 2 | The query returns the pong. | `go test ./internal/ping/application/query/pingquery -run TestPingQuery -count=1` |\n" +
	"\n## Tasks\n\n" +
	"### 1. Ping domain\n\n- Side: backend\n" +
	"- Owned files: `internal/ping/domain/pong.go`, `internal/ping/domain/legacy.go`\n" +
	"- Test first: `pong_test.go`: `TestPong` fails, because `Pong` does not exist.\n" +
	"- DoD: criterion 1.\n- Depends on: none\n" +
	"\n### 2. Ping query\n\n- Side: backend\n" +
	"- Owned files: `internal/ping/module.go`\n" +
	"- Test first: `handler_test.go`: `TestPingQuery` fails, because the handler does not exist.\n" +
	"- DoD: criterion 2.\n- Depends on: task 1\n" +
	"\n## Decisions\n\n- 2026-11-08: Word (lead): the word is pong.\n" +
	"\n## Signals\n\n" +
	"| Criterion | Signal | Source | Query | Breach | Window | Panel |\n|---|---|---|---|---|---|---|\n" +
	"| 2 | Ping latency | prometheus | `ping_latency_p99` | `> 0.5` | `5m` | Ping board |\n" +
	"\n## Tech specs\n\n" +
	"### Ping domain\n\nCode: task 1.\nCriteria: 1\n\nThe domain gives the word.\n\n" +
	"```mermaid\nstateDiagram-v2\n    [*] --> Idle\n    Idle --> Pong : ping\n```\n\n" +
	"### Ping screen\n\nCode: task 2.\nCriteria: 2 and 9\n\nThe screen lists the pings. ![The list](ui/list.excalidraw)\n\n" +
	"### Final code tree\n\n```tree\nharness/\n└── internal/\n    └── ping/\n" +
	"        ├── module.go                      changed\n" +
	"        └── domain/\n" +
	"            ├── legacy.go                  deleted\n" +
	"            └── pong.go                    new\n```\n" +
	"\n### Final test tree\n\n```tree\nharness/\n└── internal/\n    └── ping/\n" +
	"        └── domain/\n            └── pong_test.go               new\n```\n" +
	"\n### Commits\n\n1. `feat: add the pong domain` (task 1)\n2. `feat: add the ping query` (task 2)\n" +
	"\n### PR sequence\n\n1. `feat/ping-query` (this run), based on `main`: the pong domain and the ping query.\n"

var (
	scriptBlock = regexp.MustCompile(`(?s)<script\b[^>]*>.*?</script>`)
	linkAttr    = regexp.MustCompile(`\b(?:src|href)="([^"]*)"`)
	cssURL      = regexp.MustCompile(`url\(\s*["']?([^"')]*)`)
	problemRow  = regexp.MustCompile(`(?s)<li class="problem">.*?</li>`)
	idAttr      = regexp.MustCompile(`\bid="([^"]+)"`)
	navGroup    = regexp.MustCompile(`(?s)data-nav-group="([^"]+)"(.*?)</ol>`)
	sectionTag  = regexp.MustCompile(`<section id="([^"]+)"[^>]*data-group="([^"]+)"`)
	taskCard    = regexp.MustCompile(`(?s)<article id="[^"]+" class="task".*?</article>`)
)

func parsePlan(t *testing.T) domain.Document {
	t.Helper()
	doc, problems := domain.Parse(viewPlan)
	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	return doc
}

func baseInput(t *testing.T) domain.ViewInput {
	t.Helper()
	doc := parsePlan(t)
	return domain.ViewInput{
		Doc:    doc,
		Links:  domain.Trace(doc),
		Run:    domain.ViewRun{ID: "run-7", Phase: "awaiting_approval", Track: "full", Branch: "feat/ping-query"},
		Charts: map[int]string{doc.Diagrams[0].Line: "flowchart LR\n    Idle --> Pong"},
		Sketches: map[string][]byte{
			listSketch: []byte(brokenSVG),
		},
	}
}

func render(t *testing.T, in domain.ViewInput) string {
	t.Helper()
	page, err := view.NewRenderer().Render(in)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return string(page)
}

func TestPageSelfContained(t *testing.T) {
	page := render(t, baseInput(t))

	markup := scriptBlock.ReplaceAllString(page, "")
	for _, m := range linkAttr.FindAllStringSubmatch(markup, -1) {
		if !strings.HasPrefix(m[1], "#") && !strings.HasPrefix(m[1], "data:") {
			t.Errorf("attribute %q points outside the page", m[0])
		}
	}
	for _, m := range cssURL.FindAllStringSubmatch(markup, -1) {
		if !strings.HasPrefix(m[1], "data:") {
			t.Errorf("CSS url(%s points outside the page", m[1])
		}
	}
	if !strings.Contains(page, "font/woff2;base64,") || !strings.Contains(page, "mermaid") {
		t.Fatal("the page lacks the embedded fonts or the mermaid bundle")
	}
}

func idsOf(page string) []string {
	var ids []string
	for _, m := range idAttr.FindAllStringSubmatch(scriptBlock.ReplaceAllString(page, ""), -1) {
		ids = append(ids, m[1])
	}
	return ids
}

func unresolvedLinks(page string) []string {
	ids := idsOf(page)
	var missing []string
	for _, m := range linkAttr.FindAllStringSubmatch(scriptBlock.ReplaceAllString(page, ""), -1) {
		if anchor, ok := strings.CutPrefix(m[1], "#"); ok && !slices.Contains(ids, anchor) {
			missing = append(missing, m[1])
		}
	}
	return missing
}

func TestPageAnchors(t *testing.T) {
	page := render(t, baseInput(t))

	ids := idsOf(page)
	for _, want := range []string{
		"c1", "c2", "t1", "t2", "s-1", "s-2", "k1", "k2", "g1", "L1", "L94",
		"f-internal-ping-domain-pong-go", "f-internal-ping-domain-pong-test-go", "f-internal-ping-module-go",
	} {
		if !slices.Contains(ids, want) {
			t.Errorf("id %q is missing", want)
		}
	}
	slices.Sort(ids)
	if dup := slices.Compact(slices.Clone(ids)); len(dup) != len(ids) {
		t.Errorf("the page repeats an id: %v", ids)
	}
	if missing := unresolvedLinks(page); len(missing) != 0 {
		t.Errorf("links without an anchor: %v", missing)
	}
	if !strings.Contains(page, `<span class="chip broken" title="No such item">9</span>`) {
		t.Error("a spec that names criterion 9 must show a broken chip, not a link")
	}
}

func TestPageFileKeysUnique(t *testing.T) {
	in := baseInput(t)
	in.Doc, _ = domain.Parse(strings.Replace(viewPlan, "legacy.go                  deleted", "pong-go                    new", 1))
	in.Links = domain.Trace(in.Doc)

	ids := idsOf(render(t, in))

	for _, want := range []string{"f-internal-ping-domain-pong-go", "f-internal-ping-domain-pong-go-2"} {
		if !slices.Contains(ids, want) {
			t.Errorf("id %q is missing, ids = %v", want, ids)
		}
	}
}

func TestPageProblems(t *testing.T) {
	in := baseInput(t)
	in.Problems = []domain.Problem{
		{Line: 20, Rule: domain.RuleTestMissing, Message: "task 1: TestPong is not in pong_test.go"},
		{Line: 5000, Rule: domain.RuleTreeDiff, Message: "line beyond the plan"},
		{Rule: domain.RuleProofMissing, Message: "git exploded"},
	}

	page := render(t, in)

	if got := strings.Count(page, `class="problem"`); got != 3 {
		t.Fatalf("problem rows = %d, want 3", got)
	}
	rows := problemRow.FindAllString(page, -1)
	if strings.Contains(rows[2], "plan.md:0") {
		t.Errorf("a problem with no line names line 0: %s", rows[2])
	}
	for i, want := range [][]string{
		{`href="#L20"`, "test-missing", "task 1: TestPong is not in pong_test.go"},
		{"plan.md:5000", "tree-diff", "line beyond the plan"},
		{"plan.md", string(domain.RuleProofMissing), "git exploded"},
	} {
		for _, piece := range want {
			if !strings.Contains(rows[i], piece) {
				t.Errorf("problem row %d lacks %q: %s", i, piece, rows[i])
			}
		}
	}
	if strings.Contains(rows[1], "href=") {
		t.Error("a problem beyond the plan must not link to a missing line")
	}
	if missing := unresolvedLinks(page); len(missing) != 0 {
		t.Errorf("links without an anchor: %v", missing)
	}
	if strings.Index(page, `id="problems"`) > strings.Index(page, `id="trace-matrix"`) {
		t.Error("the problems must come first in the QA group")
	}

	clean := render(t, baseInput(t))
	if strings.Count(clean, `class="clean"`) != 1 || strings.Contains(clean, `class="problem"`) {
		t.Error("a clean plan must show one clean line and no problem")
	}
}

func statesOf(page, id string) []string {
	var states []string
	pattern := regexp.MustCompile(`(?:id|data-key)="` + regexp.QuoteMeta(id) + `"[^>]*data-state="([^"]*)"`)
	for _, m := range pattern.FindAllStringSubmatch(page, -1) {
		states = append(states, m[1])
	}
	return states
}

func requireStates(t *testing.T, page string, want map[string]string) {
	t.Helper()
	for id, state := range want {
		got := statesOf(page, id)
		if len(got) == 0 {
			t.Errorf("%s has no state", id)
		}
		for _, g := range got {
			if g != state {
				t.Errorf("%s state = %q, want %q", id, g, state)
			}
		}
	}
}

func TestPageVerifyState(t *testing.T) {
	in := baseInput(t)
	in.Branch = &domain.Branch{
		Files: []domain.FileChange{
			{Path: pongFile, Status: domain.StatusModified},
			{Path: moduleFile, Status: domain.StatusModified},
		},
		Commits: []domain.BranchCommit{{SHA: "a1", Subject: pongSubject}},
	}
	in.Proofs = map[int]bool{1: true, 2: false}

	page := render(t, in)

	requireStates(t, page, map[string]string{
		"c1": domain.ProofPass, "c2": domain.ProofFail,
		"f-internal-ping-domain-pong-go":      domain.FileModified,
		"f-internal-ping-domain-legacy-go":    domain.FileMissing,
		"f-internal-ping-module-go":           domain.FileModified,
		"f-internal-ping-domain-pong-test-go": domain.FileMissing,
		"k1":                                  domain.CommitFound,
		"k2":                                  domain.CommitMissing,
	})
}

func TestPageNoEvidence(t *testing.T) {
	in := baseInput(t)
	in.Run = domain.ViewRun{}

	page := render(t, in)

	requireStates(t, page, map[string]string{
		"c1": domain.ProofNotRun, "c2": domain.ProofNotRun,
		"f-internal-ping-domain-pong-go":      domain.FileAdded,
		"f-internal-ping-domain-legacy-go":    domain.FileDeleted,
		"f-internal-ping-module-go":           domain.FileModified,
		"f-internal-ping-domain-pong-test-go": domain.FileAdded,
		"k1":                                  domain.CommitMissing,
		"k2":                                  domain.CommitMissing,
	})
	if strings.Contains(page, `class="phase"`) || strings.Contains(page, "<span>run ") {
		t.Error("the header must show no run id and no phase")
	}
}

func TestPageTaskModel(t *testing.T) {
	in := baseInput(t)
	in.Doc.Tasks[0].Model = "haiku"
	page := render(t, in)

	cards := taskCard.FindAllString(page, -1)
	if len(cards) != 2 {
		t.Fatalf("task cards = %d, want 2", len(cards))
	}
	for _, card := range cards {
		switch {
		case strings.Contains(card, "<h3>Ping domain</h3>"):
			if !strings.Contains(card, "haiku") {
				t.Errorf("task 1 card lacks the model haiku")
			}
		case strings.Contains(card, "<h3>Ping query</h3>"):
			if strings.Contains(card, "haiku") {
				t.Errorf("task 2 card shows a model it does not set")
			}
		}
	}
}

func TestPageNavGroups(t *testing.T) {
	page := render(t, baseInput(t))

	var groups []string
	groupOfSection := map[string]string{}
	for _, m := range navGroup.FindAllStringSubmatch(page, -1) {
		groups = append(groups, m[1])
		for _, h := range linkAttr.FindAllStringSubmatch(m[2], -1) {
			groupOfSection[strings.TrimPrefix(h[1], "#")] = m[1]
		}
	}
	want := []string{view.GroupProduct, view.GroupSpecs, view.GroupTasks, view.GroupQA, view.GroupSignals}
	if !slices.Equal(groups, want) {
		t.Fatalf("nav groups = %v, want %v", groups, want)
	}
	sections := sectionTag.FindAllStringSubmatch(page, -1)
	if len(sections) != len(groupOfSection) {
		t.Errorf("%d sections but %d nav links", len(sections), len(groupOfSection))
	}
	for _, s := range sections {
		if groupOfSection[s[1]] != s[2] || !slices.Contains(want, s[2]) {
			t.Errorf("section %s is in group %q, nav says %q", s[1], s[2], groupOfSection[s[1]])
		}
	}
}

func TestPageSketchError(t *testing.T) {
	in := baseInput(t)
	in.Sketches = map[string][]byte{listSketch: []byte(`element "box-9": unknown kind "blob"`)}

	page := render(t, in)

	if !strings.Contains(page, `<p class="sketch-error">element &#34;box-9&#34;: unknown kind &#34;blob&#34;</p>`) {
		t.Error("the sketch error text must stand in place of the SVG")
	}
}
