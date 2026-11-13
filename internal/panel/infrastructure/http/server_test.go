package http

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net"
	nethttp "net/http"
	"net/http/httptest"
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

func (f *fakePages) HasRegion(name string) bool { return name == "criteria" }

func (f *fakePages) Region(_ context.Context, card domain.Card, name string, _ domain.Served) ([]byte, error) {
	f.calls++
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
	runs := fakeRuns{cards: cards}
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
		for _, want := range []string{`hx-get="/regions/board"`, `every 5s`, "panel serve", `href="/runs/20261113-a"`,
			"Planning", "Approval", "Working", "Reviewing", "Shipping", "Done", links.Htmx, links.Panel, "no plan &lt;b&gt;"} {
			if !strings.Contains(body, want) {
				t.Errorf("board page lacks %q", want)
			}
		}
	})

	t.Run("board region", func(t *testing.T) {
		rec := get(h, "localhost", "/regions/board")
		if rec.Code != 200 || strings.Contains(rec.Body.String(), "<html") {
			t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
		}
		version := domain.NewBoard(cards).Version
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

	t.Run("run with no plan gets the placeholder", func(t *testing.T) {
		rec := get(h, "localhost", "/runs/noplan")
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), `http-equiv="refresh" content="5"`) {
			t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
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
		var got map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if rec.Code != 200 || got["version"] != "9.9.9" || got["repo"] != "/work/repo" {
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
