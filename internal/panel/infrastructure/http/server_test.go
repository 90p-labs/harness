package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/90p-labs/harness/internal/panel/application/query/board"
	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/webkit"
)

type fakeRuns struct{ cards []domain.Card }

func (f fakeRuns) List(context.Context) ([]domain.Card, error) { return f.cards, nil }

func (f fakeRuns) Get(_ context.Context, id string) (domain.Card, error) {
	for _, c := range f.cards {
		if c.ID == id {
			return c, nil
		}
	}
	return domain.Card{}, domain.ErrNotFound
}

type fakePages struct{ calls int }

func (f *fakePages) Page(_ context.Context, card domain.Card, _ domain.Served) ([]byte, error) {
	if card.ID == "noplan" {
		return nil, domain.ErrNoPlan
	}
	if card.ID == "broken" {
		return nil, context.DeadlineExceeded
	}
	return []byte("<html>page " + card.ID + "</html>"), nil
}

func (f *fakePages) HasRegion(name string) bool { return name == "criteria" || name == "summary" }

func (f *fakePages) Region(_ context.Context, card domain.Card, name string, _ domain.Served) ([]byte, error) {
	f.calls++
	if card.ID == "noplan" {
		return nil, domain.ErrNoPlan
	}
	if name != "criteria" {
		return nil, domain.ErrNotFound
	}
	return []byte("<section>criteria " + card.ID + "</section>"), nil
}

var cards = []domain.Card{
	{ID: "20261113-a", Title: "panel serve", Phase: "working", Branch: "feat/x", PRURL: "https://example.test/pr/1",
		Version: domain.Version{Plan: "p1", Run: "r1"}},
	{ID: "noplan", Title: "no plan <b>", Phase: "planning", Version: domain.Version{Plan: "p2", Run: "r2"}},
	{ID: "broken", Title: "broken", Phase: "working", Version: domain.Version{Plan: "p3", Run: "r3"}},
}

func newServer(t *testing.T, logs *bytes.Buffer) (*Server, *fakePages) {
	t.Helper()
	return newServerWith(t, logs, cards)
}

func newServerWith(t *testing.T, logs *bytes.Buffer, list []domain.Card) (*Server, *fakePages) {
	t.Helper()
	runs := fakeRuns{cards: list}
	pages := &fakePages{}
	logger := slog.New(slog.NewTextHandler(logs, nil))
	return New(Config{
		Version: "9.9.9",
		Repo:    "/work/repo",
		Board:   board.NewHandler(runs),
		Page:    page.NewHandler(runs, pages),
		Logger:  logger,
	}), pages
}

func get(h nethttp.Handler, host, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(nethttp.MethodGet, target, nil)
	req.Host = host
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRegionMissingLogOnlyForKnownRun(t *testing.T) {
	t.Run("an unknown run answers 404 with no missing-region log", func(t *testing.T) {
		var logs bytes.Buffer
		srv, _ := newServer(t, &logs)
		if rec := get(srv.Handler(), "localhost", "/runs/gone/regions/criteria"); rec.Code != 404 {
			t.Fatalf("status %d", rec.Code)
		}
		if strings.Contains(logs.String(), "panel region missing") {
			t.Errorf("unknown run raised the missing-region log: %q", logs.String())
		}
	})
	t.Run("a known run with an unknown region logs it", func(t *testing.T) {
		var logs bytes.Buffer
		srv, _ := newServer(t, &logs)
		if rec := get(srv.Handler(), "localhost", "/runs/20261113-a/regions/nope"); rec.Code != 404 {
			t.Fatalf("status %d", rec.Code)
		}
		if !strings.Contains(logs.String(), "panel region missing") {
			t.Errorf("log %q", logs.String())
		}
	})
}

func TestRoutes(t *testing.T) {
	var logs bytes.Buffer
	srv, pages := newServer(t, &logs)
	h := srv.Handler()
	links, err := webkit.Linked()
	if err != nil {
		t.Fatalf("Linked: %v", err)
	}

	t.Run("board page polls its region", func(t *testing.T) {
		rec := get(h, "127.0.0.1:7777", "/")
		body := rec.Body.String()
		if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
			t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
		}
		for _, want := range []string{`hx-get="/regions/board"`, `every 5s`, `hx-ext="morph,sse"`, `sse-connect="/events"`,
			`sse:runs-changed, every 5s [!window.harnessLive]`, links.Sse, links.Live, links.Icon, "panel serve", `href="/runs/20261113-a"`,
			"Planning", "Approval", "Working", "Reviewing", "Shipping", "Done", links.Htmx, links.Panel, "no plan &lt;b&gt;"} {
			if !strings.Contains(body, want) {
				t.Errorf("board page lacks %q", want)
			}
		}
	})

	t.Run("board card shows the short title, the short id and the age", func(t *testing.T) {
		const id = "20261113-133205-panel-live"
		stamp, err := time.ParseInLocation("20060102-150405", id[:15], time.Local)
		if err != nil {
			t.Fatal(err)
		}
		now := stamp.Add(3*time.Hour + 5*time.Minute)
		longTitle := strings.Repeat("t", 100)
		runs := fakeRuns{cards: []domain.Card{{ID: id, Title: longTitle, Phase: "working", Version: domain.Version{Plan: "p1", Run: "r1"}}}}
		aged := New(Config{
			Version: "9.9.9",
			Repo:    "/work/repo",
			Board:   board.NewHandler(runs),
			Page:    page.NewHandler(runs, &fakePages{}),
			Logger:  slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil)),
			Now:     func() time.Time { return now },
		})
		for _, target := range []string{"/", "/regions/board"} {
			body := get(aged.Handler(), "localhost", target).Body.String()
			for _, want := range []string{
				`<span class="rid" title="` + id + `">20261113-133205</span>`,
				`<span class="age">3h</span>`,
				`title="` + longTitle + `">` + longTitle[:79] + `…</a>`,
			} {
				if !strings.Contains(body, want) {
					t.Errorf("%s lacks %q", target, want)
				}
			}
		}
	})

	t.Run("board region", func(t *testing.T) {
		rec := get(h, "localhost", "/regions/board")
		if rec.Code != 200 || strings.Contains(rec.Body.String(), "<html") {
			t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
		}
		version := domain.NewBoard(cards).Version
		root := regexp.MustCompile(`^<div([^>]*)>`).FindStringSubmatch(rec.Body.String())
		if root == nil {
			t.Fatalf("region root is no div: %q", rec.Body.String())
		}
		attrs := map[string]string{}
		for _, m := range regexp.MustCompile(`([\w-]+)="([^"]*)"`).FindAllStringSubmatch(root[1], -1) {
			attrs[m[1]] = m[2]
		}
		for name, want := range map[string]string{"class": "app", "id": "board", "hx-get": "/regions/board",
			"hx-trigger": "sse:runs-changed, every 5s [!window.harnessLive]", "hx-swap": "morph:outerHTML", "data-v": version} {
			if attrs[name] != want {
				t.Errorf("region root %s = %q, want %q", name, attrs[name], want)
			}
		}
		same := get(h, "localhost", "/regions/board?v="+version)
		if same.Code != 204 || same.Body.Len() != 0 {
			t.Fatalf("unchanged: status %d body %q", same.Code, same.Body.String())
		}
		stale := get(h, "localhost", "/regions/board?v=old")
		if stale.Code != 200 {
			t.Fatalf("stale: status %d", stale.Code)
		}
	})

	t.Run("run page", func(t *testing.T) {
		rec := get(h, "localhost", "/runs/20261113-a")
		if rec.Code != 200 || rec.Body.String() != "<html>page 20261113-a</html>" {
			t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
		}
		if rec := get(h, "localhost", "/runs/missing"); rec.Code != 404 {
			t.Fatalf("unknown run: status %d", rec.Code)
		}
	})

	t.Run("run with no plan gets the styled page", func(t *testing.T) {
		rec := get(h, "localhost", "/runs/noplan")
		body := rec.Body.String()
		if rec.Code != 200 || !strings.Contains(body, `sse-connect="/events?run=noplan"`) || strings.Contains(body, "http-equiv") {
			t.Fatalf("status %d body %q", rec.Code, body)
		}
	})

	t.Run("region versions", func(t *testing.T) {
		base := "/runs/20261113-a/regions/criteria"
		unchanged := get(h, "localhost", base+"?v=p1.r1")
		if unchanged.Code != 204 || unchanged.Body.Len() != 0 {
			t.Fatalf("unchanged: status %d", unchanged.Code)
		}
		if pages.calls != 0 {
			t.Fatalf("unchanged version rendered %d times", pages.calls)
		}
		refresh := get(h, "localhost", base+"?v=p0.r1")
		if refresh.Code != 200 || refresh.Header().Get("HX-Refresh") != "true" || refresh.Body.Len() != 0 {
			t.Fatalf("refresh: status %d header %q body %q", refresh.Code, refresh.Header().Get("HX-Refresh"), refresh.Body.String())
		}
		fresh := get(h, "localhost", base+"?v=p1.r0")
		if fresh.Code != 200 || fresh.Body.String() != "<section>criteria 20261113-a</section>" || fresh.Header().Get("HX-Refresh") != "" {
			t.Fatalf("run part stale: status %d body %q", fresh.Code, fresh.Body.String())
		}
	})

	t.Run("unknown names answer 404", func(t *testing.T) {
		for _, target := range []string{"/runs/missing/regions/criteria", "/runs/20261113-a/regions/nope?v=p1.r0",
			"/runs/20261113-a/regions/nope", "/runs/20261113-a/regions/nope?v=p1.r1", "/nowhere"} {
			if rec := get(h, "localhost", target); rec.Code != 404 {
				t.Errorf("%s: status %d", target, rec.Code)
			}
		}
	})

	t.Run("assets", func(t *testing.T) {
		rec := get(h, "localhost", links.Htmx)
		if rec.Code != 200 || rec.Body.Len() == 0 {
			t.Fatalf("status %d", rec.Code)
		}
		if got := rec.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
			t.Errorf("cache control %q", got)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Errorf("content type %q", ct)
		}
		for _, target := range []string{"/assets/000000000000/htmx.min.js", strings.Replace(links.Htmx, "htmx.min.js", "nope.js", 1)} {
			if rec := get(h, "localhost", target); rec.Code != 404 {
				t.Errorf("%s: status %d", target, rec.Code)
			}
		}
	})

	t.Run("healthz", func(t *testing.T) {
		rec := get(h, "localhost", "/healthz")
		var got map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || got["version"] != "9.9.9" || got["repo"] != "/work/repo" || got["pid"] != float64(os.Getpid()) {
			t.Fatalf("status %d body %v", rec.Code, got)
		}
	})

	t.Run("5xx is logged and no CORS header", func(t *testing.T) {
		logs.Reset()
		rec := get(h, "localhost", "/runs/broken")
		if rec.Code != 500 || !strings.Contains(logs.String(), "reason=") {
			t.Fatalf("status %d log %q", rec.Code, logs.String())
		}
		for _, target := range []string{"/", "/healthz", "/runs/20261113-a", "/regions/board", "/runs/broken"} {
			if v := get(h, "localhost", target).Header().Get("Access-Control-Allow-Origin"); v != "" {
				t.Errorf("%s sets Access-Control-Allow-Origin %q", target, v)
			}
		}
	})
}

func TestHostGuard(t *testing.T) {
	var logs bytes.Buffer
	srv, _ := newServer(t, &logs)
	h := srv.Handler()
	allowed := []string{"127.0.0.1", "127.0.0.1:8080", "localhost", "localhost:3000", "LOCALHOST:80", "LocalHost"}
	denied := []string{"", "localhost.", "localhost.:80", "127.0.0.1.", "localhost.evil.com", "127.0.0.1.evil.com",
		"[::1]", "[::1]:80", "[localhost]:80", "[127.0.0.1]:8080", "evil.com", "evil.com:80", "localhost:80:90", "localhost:abc", "localhost:", "127.0.0.1:", "0.0.0.0:80", "foo.localhost"}
	routes := []string{"/", "/regions/board", "/runs/20261113-a", "/runs/20261113-a/regions/criteria", "/assets/x/y", "/healthz", "/nowhere"}

	for _, host := range allowed {
		if rec := get(h, host, "/healthz"); rec.Code != 200 {
			t.Errorf("host %q: status %d", host, rec.Code)
		}
	}
	for _, host := range denied {
		for _, route := range routes {
			rec := get(h, host, route)
			if rec.Code != 403 {
				t.Errorf("host %q route %s: status %d", host, route, rec.Code)
			}
			if rec.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Errorf("host %q sets a CORS header", host)
			}
		}
	}
	if !strings.Contains(logs.String(), "reason=") {
		t.Errorf("refusals are not logged: %q", logs.String())
	}
}

func TestListenLoopback(t *testing.T) {
	ln, err := Listen(0)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok || !addr.IP.Equal(net.IPv4(127, 0, 0, 1)) || addr.Port == 0 {
		t.Fatalf("listener on %v", ln.Addr())
	}

	var logs bytes.Buffer
	srv, _ := newServer(t, &logs)
	if got := srv.httpServer().ReadHeaderTimeout; got != 10*time.Second {
		t.Fatalf("ReadHeaderTimeout %v", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln) }()

	res, err := nethttp.Get("http://" + ln.Addr().String() + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve returned %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not stop on cancel")
	}
}

var longToken = strings.Repeat("x", 300)

var shellCards = []domain.Card{
	{ID: "20261113-a", Title: "panel serve", Phase: "working", Branch: "feat/x", PRURL: "https://example.test/pr/1",
		Version: domain.Version{Plan: "p1", Run: "r1"}},
	{ID: "20261113-b", Title: "needs an approval", Phase: "awaiting_approval", Version: domain.Version{Plan: "p2", Run: "r2"}},
	{ID: "20261113-c", Title: longToken, Phase: "blocked", BlockedFrom: "working", Branch: "feat/" + longToken,
		PRURL: "javascript:alert(1)", Version: domain.Version{Plan: "p3", Run: "r3"}},
}

func boardBody(t *testing.T, list []domain.Card) string {
	t.Helper()
	var logs bytes.Buffer
	srv, _ := newServerWith(t, &logs, list)
	rec := get(srv.Handler(), "localhost", "/")
	if rec.Code != 200 {
		t.Fatalf("status %d", rec.Code)
	}
	return rec.Body.String()
}

func themeKey(t *testing.T, src string) string {
	t.Helper()
	m := regexp.MustCompile(`THEME_KEY\s*=\s*'([^']+)'`).FindStringSubmatch(src)
	if m == nil {
		t.Fatal("no THEME_KEY")
	}
	return m[1]
}

func TestBoardShell(t *testing.T) {
	body := boardBody(t, shellCards)
	for _, want := range []string{
		`<div class="app" id="board"`, `<aside class="side" aria-label="Board"`, `<nav class="nav" aria-label="Columns"`, `class="ws"`,
		`<span>Runs</span><span class="sep">/</span><b>Board</b>`,
		`<main>`, `class="strip"`, `data-theme-toggle data-morph-keep`,
		`<a href="#col-planning">Planning<span class="n">0</span></a>`,
		`<a href="#col-approval">Approval<span class="n bad">1</span></a>`,
		`<a href="#col-working">Working<span class="n">2</span></a>`,
		string(webkit.Stylesheet()), boardCSS,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("board page lacks %q", want)
		}
	}
	for _, key := range []string{"planning", "approval", "working", "reviewing", "shipping", "done"} {
		if !strings.Contains(body, `<div class="col" id="col-`+key+`">`) {
			t.Errorf("no column div for %s", key)
		}
	}
	for _, bad := range []string{"<section", `class="page"`} {
		if strings.Contains(body, bad) {
			t.Errorf("board page holds %q", bad)
		}
	}
	_, afterBody, ok := strings.Cut(body, "<body")
	if !ok {
		t.Fatal("board page has no <body")
	}
	if strings.Contains(afterBody, `<h1`) {
		t.Errorf("board page keeps the old header")
	}
	if strings.Contains(boardBody(t, nil), `n bad`) {
		t.Errorf("an empty approval column needs no attention")
	}
	view, err := os.ReadFile("../../../plan/infrastructure/view/view.js")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := themeKey(t, boardJS), themeKey(t, string(view)); got != want {
		t.Errorf("board.js theme key %q, view.js %q", got, want)
	}
	if !strings.Contains(boardJS, "data-theme-toggle") {
		t.Errorf("board.js does not drive the toggle")
	}
}

func TestBoardCard(t *testing.T) {
	body := boardBody(t, shellCards)
	for _, want := range []string{
		`<article class="task">`,
		`<span class="rid" title="20261113-a">20261113-a</span>`,
		`<h3><a href="/runs/20261113-a" title="panel serve">panel serve</a></h3>`,
		`<span class="chip" title="feat/x"><span class="chip-t">feat/x</span></span>`,
		`<a class="chip pr" href="https://example.test/pr/1" title="https://example.test/pr/1" target="_blank" rel="noopener noreferrer"><span class="chip-t">PR #1</span></a>`,
		`<h3><a href="/runs/20261113-c" title="` + longToken + `">` + longToken[:79] + `…</a></h3>`,
		`<span class="st st-fail">blocked</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("board page lacks %q", want)
		}
	}
	if strings.Contains(body, `href="javascript`) || !strings.Contains(body, "#ZgotmplZ") {
		t.Errorf("an unsafe PR url became a live link")
	}
	// The column names the phase, so only a blocked card carries a badge.
	if strings.Contains(body, `<span class="st">`) {
		t.Errorf("a card shows a plain phase badge")
	}
	// The kit styles .lbl as a bordered pill, so a chip must not use it.
	if strings.Contains(body, `class="lbl"`) {
		t.Errorf("a board chip uses the kit class lbl, which draws a pill inside the chip")
	}
	// A PR url with no trailing number keeps the plain label.
	if !strings.Contains(body, `<span class="chip-t">PR</span>`) {
		t.Errorf("the unsafe PR chip lacks its plain chip-t label")
	}
}

// TestBoardCardWholeClick holds the card a single target: the kit lifts the
// whole card on hover, so the title link stretches over the card, and only
// the PR chip stays above it as its own link.
func TestBoardCardWholeClick(t *testing.T) {
	r := parseRules(t, boardCSS)
	want := []struct{ sel, prop, value string }{
		{".task", "position", "relative"},
		{".task h3 a::after", "content", `""`},
		{".task h3 a::after", "position", "absolute"},
		{".task h3 a::after", "inset", "0"},
		{".task h3 a::after", "border-radius", "var(--r)"},
		{".task h3 a:focus-visible", "outline", "none"},
		{".task h3 a:focus-visible::after", "box-shadow", "0002pxvar(--accent)"},
		{".task .chip.pr", "position", "relative"},
		{".task .chip.pr", "z-index", "1"},
	}
	for _, w := range want {
		if got := r[w.sel][w.prop]; got != w.value {
			t.Errorf("%s { %s } = %q, want %q", w.sel, w.prop, got, w.value)
		}
	}
	// A positioned element between the card and the link would shrink the
	// stretched area to that element.
	for _, sel := range []string{".task h3", ".task h3 a", ".task .tid", ".task .chips"} {
		if p := r[sel]["position"]; p != "" {
			t.Errorf("%s { position: %s } traps the stretched link", sel, p)
		}
	}
}

type rules map[string]map[string]string

// parseRules reads a flat stylesheet into selector and declaration pairs.
// The file has no at-rules, so a brace pair is one rule.
func parseRules(t *testing.T, css string) rules {
	t.Helper()
	css = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	out := rules{}
	for _, block := range strings.Split(css, "}") {
		if strings.TrimSpace(block) == "" {
			continue
		}
		head, decls, ok := strings.Cut(block, "{")
		if !ok || strings.Contains(decls, "{") || strings.HasPrefix(strings.TrimSpace(head), "@") {
			t.Fatalf("board.css holds a nested or at-rule block: %q", block)
		}
		for _, sel := range strings.Split(head, ",") {
			sel = strings.Join(strings.Fields(sel), " ")
			if out[sel] == nil {
				out[sel] = map[string]string{}
			}
			for _, d := range strings.Split(decls, ";") {
				name, value, ok := strings.Cut(d, ":")
				if !ok {
					continue
				}
				out[sel][strings.TrimSpace(name)] = strings.Join(strings.Fields(value), "")
			}
		}
	}
	return out
}

func TestBoardNoPageOverflow(t *testing.T) {
	r := parseRules(t, boardCSS)
	want := []struct{ sel, prop, value string }{
		{"body", "overflow-x", "clip"},
		{"#board > div", "min-width", "0"},
		{"#board main", "max-width", "none"},
		{"#board main", "padding", "24px"},
		{".strip", "display", "grid"},
		{".strip", "grid-auto-flow", "column"},
		{".strip", "grid-auto-columns", "minmax(150px,1fr)"},
		{".strip", "overflow-x", "auto"},
		{".col", "min-width", "0"},
		{".task", "min-width", "0"},
		{".task", "overflow-wrap", "anywhere"},
		{".task h3", "-webkit-line-clamp", "2"},
		{".task h3", "line-clamp", "2"},
		{".task h3", "overflow", "hidden"},
		{".chip", "max-width", "100%"},
		{".chip .chip-t", "text-overflow", "ellipsis"},
		{".chip .chip-t", "overflow", "hidden"},
		{".chip .chip-t", "white-space", "nowrap"},
		{".task .tid .rid", "min-width", "0"},
		{".task .tid .rid", "text-overflow", "ellipsis"},
		{".task .tid .rid", "overflow", "hidden"},
		{".task .tid .rid", "white-space", "nowrap"},
		{".chip.pr", "color", "var(--accent)"},
		{".chip.pr", "background", "var(--accent-bg)"},
	}
	for _, w := range want {
		if got := r[w.sel][w.prop]; got != w.value {
			t.Errorf("%s { %s } = %q, want %q", w.sel, w.prop, got, w.value)
		}
	}
	if regexp.MustCompile(`:\s*#[0-9a-fA-F]{3,8}\b`).MatchString(boardCSS) {
		t.Errorf("board.css holds a literal color")
	}
	for sel, decls := range r {
		if decls["overflow-x"] == "auto" && sel != ".strip" {
			t.Errorf("%s scrolls sideways: only the strip may", sel)
		}
	}
}
