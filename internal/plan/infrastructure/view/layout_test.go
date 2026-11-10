package view_test

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
)

var (
	approveBtn = regexp.MustCompile(`(?s)<button[^>]*data-act="approve"[^>]*>(.*?)</button>`)
	sendBtn    = regexp.MustCompile(`<button[^>]*data-act="send-corrections"[^>]*>`)
	spaces     = regexp.MustCompile(`\s+`)
)

func TestPageThreePane(t *testing.T) {
	page := render(t, baseInput(t))

	for _, want := range []string{
		`<header class="top"`, `<aside class="side"`, `<nav id="nav"`, `<main`, `<aside class="props"`,
		`id="open-pal"`, `id="top-status"`, `id="top-actions"`, `id="peek"`, `id="peek-k"`, `id="peek-body"`,
		`data-act="close-peek"`, `id="scrim"`, `id="pal-in"`, `id="pal-list"`, `id="toast"`, `data-theme-toggle`,
		`id="tracer"`, `id="plan-data"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page has no %s", want)
		}
	}
	if networkURL.MatchString(page) {
		t.Errorf("the page loads from the network: %s", networkURL.FindString(page))
	}

	_, in := programInput(t)
	programPage := render(t, in)
	for _, want := range []string{`<header class="top"`, `<aside class="side"`, `<main`} {
		if !strings.Contains(programPage, want) {
			t.Errorf("the program page has no %s", want)
		}
	}
	if networkURL.MatchString(programPage) {
		t.Errorf("the program page loads from the network")
	}
}

func TestPageApprovalReply(t *testing.T) {
	in := baseInput(t)
	page := render(t, in)

	if !strings.Contains(page, `id="reply"`) {
		t.Error("awaiting_approval has no reply box")
	}
	if !sendBtn.MatchString(page) {
		t.Error("awaiting_approval has no send-corrections button")
	} else if !strings.Contains(sendBtn.FindString(page), "disabled") {
		t.Error("send-corrections must start disabled")
	}
	buttons := approveBtn.FindAllStringSubmatch(page, -1)
	if len(buttons) == 0 {
		t.Fatal("awaiting_approval has no approve button")
	}
	for _, m := range buttons {
		text := spaces.ReplaceAllString(m[1], " ")
		if !strings.Contains(text, "Approve") || !strings.Contains(text, "<kbd>⌘↵</kbd>") {
			t.Errorf("approve button must read Approve with a ⌘↵ kbd, got %q", text)
		}
	}

	for _, phase := range []string{"awaiting_approval", "working", "done", "planning"} {
		in.Run.Phase = phase
		out := render(t, in)
		if strings.Contains(out, "harness run") || strings.Contains(out, "Next step") {
			t.Errorf("phase %s shows CLI text", phase)
		}
		if phase == "awaiting_approval" {
			continue
		}
		for _, banned := range []string{`id="reply"`, `data-act="approve"`, `data-act="send-corrections"`} {
			if strings.Contains(out, banned) {
				t.Errorf("phase %s must not hold %s", phase, banned)
			}
		}
	}
}

func TestPageResponsive(t *testing.T) {
	raw, err := os.ReadFile("view.css")
	if err != nil {
		t.Fatal(err)
	}
	css := spaces.ReplaceAllString(string(raw), " ")
	compact := strings.ReplaceAll(css, " ", "")

	for _, width := range []string{"1180px", "860px"} {
		media := regexp.MustCompile(`@media\(max-width:` + width + `\)\{`)
		if !media.MatchString(compact) {
			t.Errorf("view.css has no @media (max-width: %s) rule", width)
		}
	}
	for _, want := range []string{
		"grid-template-columns:minmax(0,1fr)",
		"overflow-wrap:anywhere",
		"min-width:0",
	} {
		if !strings.Contains(compact, want) {
			t.Errorf("view.css has no %s", want)
		}
	}
	if !regexp.MustCompile(`\.task\{[^}]*min-width:0`).MatchString(compact) {
		t.Error("task cards must set min-width:0")
	}
	if !regexp.MustCompile(`\.board\{[^}]*grid-template-columns:repeat\(3,minmax\(0,1fr\)\)`).MatchString(compact) {
		t.Error("the board must use three minmax(0, 1fr) columns")
	}
	if !regexp.MustCompile(`@media\(max-width:860px\)\{[^@]*\.app\{[^}]*grid-template-columns:minmax\(0,1fr\)`).MatchString(compact) {
		t.Error("the app must fold to one column at 860px")
	}
}

func TestPageQAFixes(t *testing.T) {
	page := render(t, baseInput(t))
	css, err := os.ReadFile("view.css")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("task labels do not reuse the sidebar class", func(t *testing.T) {
		if n := strings.Count(page, `class="side"`); n != 1 {
			t.Errorf("the page has %d elements with class side, want only the sidebar", n)
		}
	})
	t.Run("the change legend has colors", func(t *testing.T) {
		for _, want := range []string{".lg-added i", ".lg-modified i", ".lg-deleted i"} {
			if !strings.Contains(string(css), want) {
				t.Errorf("view.css has no %s rule", want)
			}
		}
	})
	t.Run("proof rows carry their criterion key", func(t *testing.T) {
		if !regexp.MustCompile(`<ul class="proofs">\s*<li data-key="c1"`).MatchString(page) {
			t.Error("a proof row has no data-key")
		}
	})
	t.Run("each wave is one column of a single board", func(t *testing.T) {
		if n := strings.Count(page, `class="board"`); n != 1 {
			t.Errorf("the page has %d boards, want 1", n)
		}
		cols := regexp.MustCompile(`<div class="col"><div class="col-h">Wave \d+<span class="n">\d+</span></div>`).FindAllString(page, -1)
		if len(cols) == 0 {
			t.Error("no wave column with a header and a count")
		}
		if strings.Contains(page, `class="waves"`) {
			t.Error("the waves still stack outside the board")
		}
	})
	t.Run("the file tree rows are compact", func(t *testing.T) {
		if strings.Contains(page, `<p class="tree-h">Final code tree</p><pre>`) {
			t.Error("the tree rows sit in a pre, so each newline adds a blank line")
		}
		if !strings.Contains(page, `<div class="rows">`) {
			t.Error("the tree rows have no rows container")
		}
		if regexp.MustCompile(`class="st st-"`).MatchString(page) {
			t.Error("a tree row renders an empty state badge")
		}
		if !strings.Contains(string(css), ".tr .mark") {
			t.Error("view.css does not draw the change mark as a dot")
		}
	})
	t.Run("the task card folds its owned files", func(t *testing.T) {
		if !regexp.MustCompile(`<details class="owned"><summary>\d+ files?</summary>`).MatchString(page) {
			t.Error("the owned files are not folded behind a file count")
		}
	})
	t.Run("a proof row is one flat line like the prototype", func(t *testing.T) {
		row := regexp.MustCompile(`<li data-key="c1"><span class="ico[^"]*"[^>]*>.*?</span><span class="cn-mini">1</span><code class="cmd" title="[^"]+">[^<]+</code><button type="button" class="iconbtn"[^>]*data-copy="[^"]+"[^>]*aria-label="Copy the command"><svg`)
		if !row.MatchString(page) {
			t.Error("a proof row is not an icon, a number, the command and a copy icon button")
		}
		if strings.Contains(page, `<div class="proof">`) {
			t.Error("a proof still sits in a boxed chip")
		}
	})
	t.Run("a clean plan has no red problems box", func(t *testing.T) {
		if strings.Contains(page, `<p class="clean">`) && !strings.Contains(page, `class="problems ok"`) {
			t.Error("the problems box stays red when there are no problems")
		}
		if !strings.Contains(string(css), ".problems.ok") {
			t.Error("view.css has no calm style for a clean plan")
		}
	})
	t.Run("an empty signals table is not drawn", func(t *testing.T) {
		in := baseInput(t)
		in.Doc.Signals = nil
		out := render(t, in)
		if strings.Contains(out, `<table class="signals">`) {
			t.Error("the page draws a signals table with no rows")
		}
	})
	t.Run("a matrix row keeps the criterion on one line", func(t *testing.T) {
		if !regexp.MustCompile(`<tr data-key="c1"[^>]*><th scope="row"><a href="#c1" data-key="c1" title="[^"]+">`).MatchString(page) {
			t.Error("a truncated criterion has no title with the full text")
		}
		if !regexp.MustCompile(`\.matrix tbody th a \{[^}]*text-overflow: ellipsis`).Match(css) {
			t.Error("view.css does not truncate the criterion text")
		}
	})
}

func TestPagePropertiesPane(t *testing.T) {
	in := baseInput(t)
	in.Proofs = map[int]bool{1: true}
	in.Branch = &domain.Branch{Commits: []domain.BranchCommit{{SHA: "a", Subject: pongSubject}}}
	page := render(t, in)

	props := regexp.MustCompile(`(?s)<aside class="props".*?</aside>`).FindString(page)
	if props == "" {
		t.Fatal("the page has no properties pane")
	}
	flat := spaces.ReplaceAllString(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(props, " "), " ")
	for _, want := range []string{
		"Phase Waiting for approval", "Track full", "Branch feat/ping-query", "Base main",
		"1 of 2 proved", "1 of 2 committed", "New 2", "Changed 1",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("the properties pane has no %q, text: %s", want, flat)
		}
	}
	if strings.Contains(flat, "Status") {
		t.Error("the properties pane still labels the phase Status")
	}
}

var sideFoot = regexp.MustCompile(`(?s)<div class="side-foot">(.*?)</aside>`)

func TestPageSidebarFootKeyHints(t *testing.T) {
	page := render(t, baseInput(t))
	m := sideFoot.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("the page has no sidebar foot")
	}
	foot := m[1]
	for _, want := range []string{
		`<kbd>j</kbd>`, `<kbd>k</kbd>`, `<kbd>↵</kbd>`, `<kbd>g</kbd>`,
		`<kbd>c</kbd>`, `<kbd>t</kbd>`, `<kbd>f</kbd>`,
	} {
		if !strings.Contains(foot, want) {
			t.Errorf("the sidebar foot has no %s", want)
		}
	}
}

var (
	noteGroup = regexp.MustCompile(`(?s)<details class="grp"[^>]*><summary>(.*?)</summary>`)
	tagText   = regexp.MustCompile(`<[^>]*>`)
)

func TestPageNotesAreCollapsibleGroups(t *testing.T) {
	in := baseInput(t)
	doc, _ := domain.Parse(viewPlan + "\n## Review risks\n\n- Late reply. Test: `TestPong`.\n- Slow reply. Test: `TestSlow`.\n")
	in.Doc = doc
	in.Links = domain.Trace(doc)
	page := render(t, in)

	section := regexp.MustCompile(`(?s)<section id="notes".*?</section>`).FindString(page)
	if section == "" {
		t.Fatal("the page has no notes section")
	}
	var got []string
	for _, m := range noteGroup.FindAllStringSubmatch(section, -1) {
		got = append(got, spaces.ReplaceAllString(strings.TrimSpace(tagText.ReplaceAllString(m[1], " ")), " "))
	}
	want := []string{"Decisions 1", "Review risks 2"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("note group summaries = %q, want %q", got, want)
	}
	if strings.Contains(regexp.MustCompile(`<details class="grp"[^>]*><summary>[^<]*<svg[^>]*>.*?</svg>Decisions`).FindString(section), " open") {
		t.Error("the Decisions group opens by default")
	}
	if !regexp.MustCompile(`<details class="grp" open><summary>[^<]*<svg[^>]*>.*?</svg>Review risks`).MatchString(section) {
		t.Error("the Review risks group does not open by default")
	}
	for _, empty := range []string{"Threat model", "Risks", "Non-goals", "Edge cases"} {
		if strings.Contains(section, empty) {
			t.Errorf("the notes draw the empty group %q", empty)
		}
	}
}

func TestPageFilesToolbarAndFolders(t *testing.T) {
	page := render(t, baseInput(t))
	section := regexp.MustCompile(`(?s)<section id="trees".*?</section>`).FindString(page)
	if section == "" {
		t.Fatal("the page has no files section")
	}
	if !strings.Contains(section, `id="ftool"`) {
		t.Error("the files section has no filter toolbar")
	}
	for _, f := range []string{"all", "new", "changed", "deleted"} {
		btn := regexp.MustCompile(`<button[^>]*class="fbtn[^"]*"[^>]*data-filter="` + f + `"[^>]*>`)
		if !btn.MatchString(section) {
			t.Errorf("the toolbar has no filter button for %q", f)
		}
	}
	if !regexp.MustCompile(`<button[^>]*data-filter="all"[^>]*aria-pressed="true"`).MatchString(section) {
		t.Error("the All filter is not pressed by default")
	}
	if !regexp.MustCompile(`data-filter="deleted"[^>]*>.*?Deleted <span class="n">\d+</span>`).MatchString(section) {
		t.Error("the Deleted filter shows no count")
	}
	folder := regexp.MustCompile(`<span class="tr dir-row"[^>]*aria-expanded="true"[^>]*data-depth="\d+"|<span class="tr dir-row"[^>]*data-depth="\d+"[^>]*aria-expanded="true"`)
	if !folder.MatchString(section) {
		t.Error("the tree has no collapsible folder row, open by default")
	}
	if !regexp.MustCompile(`<span class="tr file ch-\w+"[^>]*data-change="\w+"`).MatchString(section) {
		t.Error("file rows carry no data-change for the filter")
	}
	if regexp.MustCompile(`class="tr[^"]*"[^>]*\bhidden\b`).MatchString(section) {
		t.Error("a row is hidden before the script runs")
	}
}

func TestPageReplyCardTip(t *testing.T) {
	page := render(t, baseInput(t))

	card := regexp.MustCompile(`(?s)<div class="next" id="next">.*?</div>\s*</div>|<div class="next" id="next">.*?<div class="acts">.*?</div>.*?</div>`).FindString(page)
	if card == "" {
		t.Fatal("the page has no reply card")
	}
	if !strings.Contains(card, "Tip: open an item and use Add correction to quote its ID.") || !strings.Contains(card, "<kbd>⌘↵</kbd> sends.") {
		t.Errorf("the reply card has no tip line: %s", card)
	}
	if !strings.Contains(card, "“C3: also cover the empty-diff case”") {
		t.Error("the placeholder has no curly quotes")
	}
}

func TestPageTaskCardIdAndSideDot(t *testing.T) {
	page := render(t, baseInput(t))

	if !strings.Contains(page, `<span class="tn">T1</span>`) {
		t.Error("the task card shows a bare number, not T1")
	}
	if !regexp.MustCompile(`<span class="lbl"><i class="sdot sdot-backend" aria-hidden="true"></i>backend</span>`).MatchString(page) {
		t.Error("the task side has no dot")
	}
	css, err := os.ReadFile("view.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), ".sdot-backend") {
		t.Error("view.css colors no side dot")
	}
}

func TestPageCommitsOpenWithBranchToBase(t *testing.T) {
	page := render(t, baseInput(t))

	section := regexp.MustCompile(`(?s)<section id="delivery".*?</section>`).FindString(page)
	route := regexp.MustCompile(`(?s)</h2>\s*<p class="route"><code>feat/ping-query</code> to <code>main</code></p>\s*<ol class="commits">`)
	if !route.MatchString(section) {
		t.Errorf("the commits do not open with the branch to base line: %.300s", section)
	}
}

func TestPageNavMatchesSectionOrder(t *testing.T) {
	page := render(t, baseInput(t))

	nav := regexp.MustCompile(`(?s)<nav id="nav".*?</nav>`).FindString(page)
	var navOrder, pageOrder []string
	for _, m := range regexp.MustCompile(`<a href="#([^"]+)"`).FindAllStringSubmatch(nav, -1) {
		navOrder = append(navOrder, m[1])
	}
	main := regexp.MustCompile(`(?s)<main>.*?</main>`).FindString(page)
	for _, m := range regexp.MustCompile(`<section id="([^"]+)"`).FindAllStringSubmatch(main, -1) {
		pageOrder = append(pageOrder, m[1])
	}
	var shared []string
	for _, id := range pageOrder {
		if slices.Contains(navOrder, id) {
			shared = append(shared, id)
		}
	}
	var navShared []string
	for _, id := range navOrder {
		if slices.Contains(pageOrder, id) {
			navShared = append(navShared, id)
		}
	}
	if strings.Join(shared, ",") != strings.Join(navShared, ",") {
		t.Errorf("the nav order %v differs from the page order %v", navShared, shared)
	}
}

func TestPagePropertiesHideBaseWithoutPRBase(t *testing.T) {
	doc, problems := domain.Parse(strings.Replace(viewPlan, ", based on `main`", "", 1))
	if len(problems) != 0 {
		t.Fatalf("parse problems: %v", problems)
	}
	in := baseInput(t)
	in.Doc = doc
	in.Links = domain.Trace(doc)

	props := regexp.MustCompile(`(?s)<aside class="props".*?</aside>`).FindString(render(t, in))
	flat := spaces.ReplaceAllString(regexp.MustCompile(`<[^>]*>`).ReplaceAllString(props, " "), " ")

	if !strings.Contains(flat, "Branch feat/ping-query") {
		t.Errorf("the properties pane lost the branch: %s", flat)
	}
	if strings.Contains(flat, "Base") {
		t.Errorf("the properties pane shows a Base row without a PR base: %s", flat)
	}
}

func TestPageSelectedRowHasAccentBar(t *testing.T) {
	css, err := os.ReadFile("view.css")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`\.row\.sel::before \{[^}]*width: 2px[^}]*background: var\(--accent\)`).Match(css) {
		t.Error("a selected row has no 2px accent bar")
	}
}

func TestPageCriterionRowHasProofChip(t *testing.T) {
	page := render(t, baseInput(t))

	row := regexp.MustCompile(`(?s)<li id="c1".*?</li>`).FindString(page)
	chip := regexp.MustCompile(`<a class="chip proof" href="#proofs" title="go test \./internal/ping/domain -run TestPong -count=1">`)
	if !chip.MatchString(row) {
		t.Errorf("the criterion row has no proof chip: %s", row)
	}
	if !strings.Contains(row, `<span class="id">C1</span>`) {
		t.Errorf("the criterion row shows a bare number, not C1: %s", row)
	}
}

func TestPageNoProblemsIsGreenBanner(t *testing.T) {
	css, err := os.ReadFile("view.css")
	if err != nil {
		t.Fatal(err)
	}
	rule := regexp.MustCompile(`\.problems\.ok \{[^}]*\}`).FindString(string(css))
	if !strings.Contains(rule, "var(--green)") || strings.Contains(rule, "background: none") {
		t.Errorf("a plan with no problems has no green banner: %s", rule)
	}
}

func TestPageStatusPillHasIcon(t *testing.T) {
	in := baseInput(t)
	for phase, icon := range map[string]string{"done": "pass", "working": "work", "blocked": "fail", "planning": "open"} {
		in.Run.Phase = phase
		pill := regexp.MustCompile(`(?s)<span class="status-pill" id="top-status">.*?</span>\s*</span>|<span class="status-pill" id="top-status">.*?</svg></span>[^<]*</span>`).FindString(render(t, in))
		if !strings.Contains(pill, `class="ico `+icon+`"`) {
			t.Errorf("phase %s: the status pill has no %s icon: %s", phase, icon, pill)
		}
	}
	tmpl, err := os.ReadFile("page.html.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tmpl), `"building"`) {
		t.Error("the template still tests the dead building phase")
	}
}

func TestProgramPageHasNoDeadPalette(t *testing.T) {
	_, in := programInput(t)
	page := render(t, in)
	if strings.Contains(page, `id="open-pal"`) {
		t.Error("the program page has a Jump to button, but it has no palette")
	}
}
