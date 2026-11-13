package view_test

import (
	"errors"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/90p-labs/harness/internal/plan/domain"
	"github.com/90p-labs/harness/internal/plan/infrastructure/view"
	"github.com/90p-labs/harness/internal/shared/infrastructure/webkit"
)

const (
	servedVersion = "p1.r1"
	servedBase    = "/runs/run-7/regions/"
)

func servedInput(t *testing.T) domain.ViewInput {
	t.Helper()
	in := baseInput(t)
	in.Served = &domain.ViewServed{Version: servedVersion, Base: servedBase, Events: servedEvents}
	return in
}

const (
	servedEvents  = "/events?run=run-7"
	servedTrigger = `sse:run-updated, every 5s [!window.harnessLive]`
)

func TestServedPage(t *testing.T) {
	page := render(t, servedInput(t))
	links, err := webkit.Linked()
	if err != nil {
		t.Fatalf("Linked: %v", err)
	}

	for _, url := range []string{links.Mermaid, links.Htmx, links.Morph, links.Sse, links.Panel} {
		if !strings.Contains(page, `<script src="`+url+`"></script>`) {
			t.Errorf("page does not load %s", url)
		}
	}
	if strings.Index(page, links.Htmx) > strings.Index(page, links.Sse) {
		t.Error("the sse script loads before htmx")
	}
	if !strings.Contains(page, `<link rel="stylesheet" href="`+links.Live+`">`) {
		t.Error("page does not link live.css")
	}
	if !strings.Contains(page, `<link rel="icon" href="`+links.Icon+`" type="image/svg+xml">`) {
		t.Error("page does not link the icon")
	}
	if !strings.Contains(page, `class="live-dot"`) {
		t.Error("page has no live dot")
	}
	if !strings.Contains(page, `<textarea class="reply" id="reply" data-morph-keep`) {
		t.Error("the reply textarea lacks data-morph-keep")
	}
	if !strings.Contains(page, "/assets/") || !strings.Contains(page, "url(/assets/") {
		t.Error("page does not link the fonts")
	}
	if strings.Contains(page, "@font-face{font-family:\"Inter\";src:url(data:") || strings.Contains(page, "url(data:") {
		t.Error("served page inlines a font")
	}
	body := regexp.MustCompile(`<body[^>]*>`).FindString(page)
	if body != `<body hx-ext="morph,sse" sse-connect="`+servedEvents+`">` {
		t.Errorf("body tag is %s", body)
	}
	if strings.Contains(body, "hx-get") || strings.Contains(body, "hx-swap") {
		t.Error("the body is a swap target")
	}

	names := map[string]bool{}
	for _, reg := range view.Regions() {
		names[reg.Name] = true
	}
	for _, want := range []string{"nav", "props", "top-actions"} {
		if !names[want] {
			t.Errorf("registry lacks region %q", want)
		}
	}
	for _, m := range regexp.MustCompile(`hx-get="([^"]*)"`).FindAllStringSubmatch(page, -1) {
		name, ok := strings.CutPrefix(m[1], servedBase)
		if !ok || !names[name] {
			t.Errorf("hx-get %q names no registered region", m[1])
		}
	}

	for _, reg := range view.Regions() {
		attrs := `hx-get="` + servedBase + reg.Name + `" hx-trigger="` + servedTrigger + `" hx-swap="morph:outerHTML" data-v="` + servedVersion + `"`
		root := regexp.MustCompile(`<[a-z]+[^>]*\bid="` + regexp.QuoteMeta(reg.ID) + `"[^>]*>`).FindString(page)
		if root == "" {
			t.Errorf("region %q has no root with id %q in the page", reg.Name, reg.ID)
			continue
		}
		if !strings.Contains(root, attrs) {
			t.Errorf("region %q root %s lacks %s", reg.Name, root, attrs)
		}
		out, err := view.NewRenderer().RenderRegion(servedInput(t), reg.Name)
		if err != nil {
			t.Errorf("RenderRegion(%q): %v", reg.Name, err)
			continue
		}
		if !strings.Contains(string(out), attrs) {
			t.Errorf("fragment %q lacks the poll attributes", reg.Name)
		}
	}
}

func TestServedPageWithoutEventsKeepsPoll(t *testing.T) {
	in := baseInput(t)
	in.Served = &domain.ViewServed{Version: servedVersion, Base: servedBase}
	page := render(t, in)
	if !strings.Contains(page, `<body hx-ext="morph">`) {
		t.Error("body without Events is not plain morph")
	}
	if !strings.Contains(page, `hx-trigger="every 5s"`) || strings.Contains(page, "sse:") || strings.Contains(page, "sse-connect") {
		t.Error("a page without Events must poll only and name no sse")
	}
	links, err := webkit.Linked()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, `<link rel="icon" href="`+links.Icon+`" type="image/svg+xml">`) {
		t.Error("a served page without Events does not link the icon")
	}
}

func TestServedPageRunsCrumbLinksHome(t *testing.T) {
	in := baseInput(t)
	in.Served = &domain.ViewServed{Version: servedVersion, Base: servedBase, Home: "/"}
	page := render(t, in)
	if !strings.Contains(page, `<a href="/">Runs</a>`) {
		t.Error("served page with Home does not link the Runs crumb to Home")
	}
	if strings.Contains(page, `<span>Runs</span>`) {
		t.Error("served page with Home keeps the plain Runs label")
	}
}

func TestServedPageEmptyHomeKeepsRunsLabel(t *testing.T) {
	page := render(t, servedInput(t))
	if !strings.Contains(page, `<span>Runs</span>`) {
		t.Error("served page with an empty Home lacks the plain Runs label")
	}
	if strings.Contains(page, `<a href="">Runs</a>`) {
		t.Error("served page with an empty Home links the Runs crumb to nowhere")
	}
}

// view.js is inlined in the file mode and pinned by the golden bytes, so the
// sync after a swap lives in served.js, which only the served page carries.
func TestServedScriptSyncsAfterSwap(t *testing.T) {
	page := render(t, servedInput(t))
	raw, err := os.ReadFile("served.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(page, string(raw)) {
		t.Error("the served page does not inline served.js")
	}
	for _, want := range []string{"htmx:afterSwap", "send-corrections", "#nav a", "classList.toggle('on'"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("served.js lacks %q", want)
		}
	}
	if strings.Contains(render(t, baseInput(t)), "htmx:afterSwap") {
		t.Error("the file page carries the swap sync")
	}
}

func TestFilePageHasNoPolling(t *testing.T) {
	page := render(t, baseInput(t))
	for _, s := range []string{"hx-get", "hx-ext", "data-v=", "<script src"} {
		if strings.Contains(page, s) {
			t.Errorf("file page contains %q", s)
		}
	}
}

func TestRenderRegionErrors(t *testing.T) {
	r := view.NewRenderer()
	if _, err := r.RenderRegion(baseInput(t), "nope"); !errors.Is(err, domain.ErrUnknownRegion) {
		t.Errorf("unknown name: got %v, want ErrUnknownRegion", err)
	}
	_, program := programInput(t)
	if _, err := r.RenderRegion(program, "status"); !errors.Is(err, domain.ErrNoRegions) {
		t.Errorf("program: got %v, want ErrNoRegions", err)
	}
}

var _ domain.ViewRegionRenderer = view.Renderer{}
var _ domain.ViewRenderer = view.Renderer{}
