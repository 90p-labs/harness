package view_test

import (
	"errors"
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
	in.Served = &domain.ViewServed{Version: servedVersion, Base: servedBase}
	return in
}

func TestServedPage(t *testing.T) {
	page := render(t, servedInput(t))
	links, err := webkit.Linked()
	if err != nil {
		t.Fatalf("Linked: %v", err)
	}

	for _, url := range []string{links.Mermaid, links.Htmx, links.Morph, links.Panel} {
		if !strings.Contains(page, `<script src="`+url+`"></script>`) {
			t.Errorf("page does not load %s", url)
		}
	}
	if !strings.Contains(page, "/assets/") || !strings.Contains(page, "url(/assets/") {
		t.Error("page does not link the fonts")
	}
	if strings.Contains(page, "@font-face{font-family:\"Inter\";src:url(data:") || strings.Contains(page, "url(data:") {
		t.Error("served page inlines a font")
	}
	if !strings.Contains(page, `<body hx-ext="morph">`) {
		t.Error("body lacks hx-ext morph")
	}

	for _, reg := range view.Regions() {
		attrs := `hx-get="` + servedBase + reg.Name + `" hx-trigger="every 5s" hx-swap="morph:outerHTML" data-v="` + servedVersion + `"`
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
