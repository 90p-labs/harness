package view_test

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var (
	metaKeyHandler   = regexp.MustCompile(`(?s)(metaKey\s*\|\|\s*e\.ctrlKey|ctrlKey\s*\|\|\s*e\.metaKey)[^\n]*['"]k['"]`)
	metaEnterHandler = regexp.MustCompile(`(?s)(metaKey\s*\|\|\s*e\.ctrlKey|ctrlKey\s*\|\|\s*e\.metaKey)[^\n]*['"]Enter['"]`)
)

func TestViewScriptActions(t *testing.T) {
	raw, err := os.ReadFile("view.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)

	for _, want := range []string{
		"function openPeek", "function openPal", "function approve", "function sendCorrections", "function addCorrection",
		"Plan approved. Go ahead with the build.",
		`Please change the plan before I approve it:\n`,
		"You have unsent corrections. Approve anyway?",
		"Approval copied, paste it to the agent",
		"Corrections copied, paste it to the agent",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("view.js lacks %q", want)
		}
	}
	if !metaKeyHandler.MatchString(src) {
		t.Error("view.js lacks a metaKey or ctrlKey handler for the key k")
	}
	if !metaEnterHandler.MatchString(src) {
		t.Error("view.js lacks a metaKey or ctrlKey handler for Enter")
	}
}

var (
	peekAfterMatch = regexp.MustCompile(`querySelector\('em'\)\?\.textContent(\.trim\(\))?(\.toLowerCase\(\))?\s*===\s*'([^']+)'`)
	taskAfterChips = regexp.MustCompile(`"chips" dict "Name" "([^"]+)" "Chips" \.After`)
)

func TestPeekAfterLabelMatchesTemplate(t *testing.T) {
	js, err := os.ReadFile("view.js")
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := os.ReadFile("page.html.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	m := peekAfterMatch.FindStringSubmatch(string(js))
	if m == nil {
		t.Fatal("view.js does not compare the label of the After chips")
	}
	chipsMatch := taskAfterChips.FindStringSubmatch(string(tmpl))
	if chipsMatch == nil {
		t.Fatal("the template does not render the After chips of a task")
	}
	label := strings.TrimSpace(chipsMatch[1])
	if m[2] != "" {
		label = strings.ToLower(label)
	}
	if label != m[3] {
		t.Errorf("the peek matches the label %q but the template renders %q", m[3], chipsMatch[1])
	}
}

func TestViewScriptTracesProofRows(t *testing.T) {
	raw, err := os.ReadFile("view.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), ".proofs li") {
		t.Error("the tracer does not pick up proof rows")
	}
}

var (
	copyReplyBody = regexp.MustCompile(`(?s)function copyReply\(.*?\n  }\n`)
	toastCall     = regexp.MustCompile(`toast\(message\)`)
)

func TestCopyReplyToastsAfterWrite(t *testing.T) {
	raw, err := os.ReadFile("view.js")
	if err != nil {
		t.Fatal(err)
	}
	body := copyReplyBody.FindString(string(raw))
	if body == "" {
		t.Fatal("view.js lacks function copyReply")
	}
	then := strings.Index(body, ".then(")
	catch := strings.Index(body, ".catch(")
	ok := toastCall.FindStringIndex(body)
	if then < 0 || catch < 0 || ok == nil {
		t.Fatalf("copyReply needs .then, .catch and toast(message): then=%d catch=%d toast=%v", then, catch, ok)
	}
	if ok[0] < then || ok[0] > catch {
		t.Error("the success toast must sit inside the resolved branch, before .catch")
	}
	if !strings.Contains(body, "Copy failed") || !strings.Contains(body, ".select()") {
		t.Error("copyReply must select the reply and toast a Copy failed message")
	}
	if !regexp.MustCompile(`\.catch\(fail\)`).MatchString(body) || !regexp.MustCompile(`catch \{\s*fail\(\)`).MatchString(body) {
		t.Error("a rejected write and a missing clipboard API must both call fail")
	}
}

func TestViewScriptFilesFilterAndFolders(t *testing.T) {
	raw, err := os.ReadFile("view.js")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	for _, re := range []struct{ name, pat string }{
		{"a click handler reads the filter button", `closest\(['"][^'"]*\[data-filter\][^'"]*['"]\)`},
		{"the filter compares data-change with the chosen filter", `dataset\.change`},
		{"the filter sets aria-pressed on its buttons", `aria-pressed`},
		{"a click handler reads the folder row", `closest\(['"][^'"]*\.dir-row[^'"]*['"]\)`},
		{"the folder toggle flips aria-expanded", `aria-expanded`},
		{"the folder toggle compares depth", `dataset\.depth`},
		{"rows are hidden through the hidden property", `\.hidden\s*=`},
		{"Enter on a folder row stops before the peek handler", `stopImmediatePropagation\(\); toggleDir\(dir\)`},
	} {
		if !regexp.MustCompile(re.pat).MatchString(src) {
			t.Errorf("view.js: %s", re.name)
		}
	}
	if !regexp.MustCompile(`closest\('#ftool'\);\s*if \(!bar\) return;`).MatchString(src) {
		t.Error("view.js does not guard a missing toolbar")
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestViewScriptHandlesEveryTemplateAction(t *testing.T) {
	js := readFile(t, "view.js")
	acts := map[string]bool{}
	for _, name := range []string{"page.html.tmpl", "program.html.tmpl"} {
		for _, m := range regexp.MustCompile(`data-act="([^"]+)"`).FindAllStringSubmatch(readFile(t, name), -1) {
			acts[m[1]] = true
		}
	}
	if len(acts) == 0 {
		t.Fatal("the templates name no data-act")
	}
	for act := range acts {
		if !regexp.MustCompile(`ACTS = \{[^\n]*(['"]` + regexp.QuoteMeta(act) + `['"]|\b` + regexp.QuoteMeta(act) + `\b[,:])`).MatchString(js) {
			t.Errorf("view.js has no branch for data-act %q", act)
		}
	}
}

func TestViewScriptTitleOfHasNoDuplicateBranch(t *testing.T) {
	js := readFile(t, "view.js")
	body := regexp.MustCompile(`(?s)const titleOf = .*?\n  };`).FindString(js)
	if body == "" {
		t.Fatal("view.js lacks titleOf")
	}
	if n := strings.Count(body, "return text('h3') || data.labels[key]"); n != 1 {
		t.Errorf("titleOf returns the h3 text in %d branches, want 1", n)
	}
}

func TestViewScriptIconCopyGivesFeedback(t *testing.T) {
	js := readFile(t, "view.js")
	body := regexp.MustCompile(`(?s)const bindCopy = .*?\n  \}\);`).FindString(js)
	if body == "" {
		t.Fatal("view.js lacks bindCopy")
	}
	if !strings.Contains(body, "'copied'") || !strings.Contains(body, "toast(") {
		t.Error("an icon copy button shows no copied state and no toast")
	}
}

func TestViewScriptPaletteKeyIgnoredInReplyBox(t *testing.T) {
	js := readFile(t, "view.js")
	line := regexp.MustCompile(`[^\n]*toLowerCase\(\) === 'k'[^\n]*`).FindString(js)
	if !strings.Contains(line, "textarea") {
		t.Errorf("the palette key does not skip the reply box: %s", line)
	}
}

func TestCopyFailToastStaysUntilBlur(t *testing.T) {
	js := readFile(t, "view.js")
	body := copyReplyBody.FindString(js)
	if !regexp.MustCompile(`toast\('Copy failed[^\n]*, true\)`).MatchString(body) {
		t.Error("the Copy failed toast is not sticky")
	}
	if !regexp.MustCompile(`blur', \(\) => \{[^}]*classList\.remove\('show'\)`).MatchString(body) {
		t.Error("the sticky toast does not clear when the reply box blurs")
	}
}
