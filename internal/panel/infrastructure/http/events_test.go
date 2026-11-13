package http

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	nethttp "net/http"

	"github.com/90p-labs/harness/internal/panel/application/query/board"
	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/webkit"
)

// syncBuf is a log sink that a test reads while the server writes.
type syncBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakeSignals is a Signals port that a test drives by hand.
type fakeSignals struct {
	mu        sync.Mutex
	known     map[string]bool
	subs      map[string][]chan string
	cancelled int
	opened    chan struct{}
}

func newFakeSignals(ids ...string) *fakeSignals {
	f := &fakeSignals{known: map[string]bool{}, subs: map[string][]chan string{}, opened: make(chan struct{}, 16)}
	for _, id := range ids {
		f.known[id] = true
	}
	return f
}

func (f *fakeSignals) Subscribe(_ context.Context, run string) (<-chan string, func(), error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if run != "" && !f.known[run] {
		return nil, nil, domain.ErrNotFound
	}
	ch := make(chan string, 1)
	f.subs[run] = append(f.subs[run], ch)
	done := false
	cancel := func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		if done {
			return
		}
		done = true
		f.cancelled++
		close(ch)
	}
	f.opened <- struct{}{}
	return ch, cancel, nil
}

func (f *fakeSignals) send(run, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, ch := range f.subs[run] {
		select {
		case ch <- name:
		default:
		}
	}
}

func (f *fakeSignals) cancels() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.cancelled
}

func eventsServer(t *testing.T, sigs domain.Signals, logs *syncBuf, tweak func(*Config)) *Server {
	t.Helper()
	runs := fakeRuns{cards: cards}
	cfg := Config{
		Version: "9.9.9",
		Repo:    "/work/repo",
		Board:   board.NewHandler(runs),
		Page:    page.NewHandler(runs, &fakePages{}),
		Signals: sigs,
		Logger:  slog.New(slog.NewTextHandler(logs, nil)),
	}
	if tweak != nil {
		tweak(&cfg)
	}
	return New(cfg)
}

// frames reads the blank-line separated frames of a stream into a channel.
func frames(t *testing.T, body interface{ Read([]byte) (int, error) }) <-chan string {
	t.Helper()
	out := make(chan string, 32)
	go func() {
		defer close(out)
		sc := bufio.NewScanner(body)
		var cur []string
		for sc.Scan() {
			line := sc.Text()
			if line == "" {
				out <- strings.Join(cur, "\n")
				cur = nil
				continue
			}
			cur = append(cur, line)
		}
	}()
	return out
}

func next(t *testing.T, in <-chan string) string {
	t.Helper()
	select {
	case f, ok := <-in:
		if !ok {
			t.Fatal("stream ended")
		}
		return f
	case <-time.After(2 * time.Second):
		t.Fatal("no frame within 2s")
	}
	return ""
}

func TestEvents(t *testing.T) {
	const runUpdated = "event: run-updated\ndata: run-updated"

	t.Run("signal on connect, on change and a ping", func(t *testing.T) {
		logs := &syncBuf{}
		sigs := newFakeSignals("20261113-a")
		srv := eventsServer(t, sigs, logs, func(c *Config) { c.PingEvery = 30 * time.Millisecond })
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()

		res, err := nethttp.Get(ts.URL + "/events?run=20261113-a")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/event-stream") ||
			res.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("status %d type %q cache %q", res.StatusCode, res.Header.Get("Content-Type"), res.Header.Get("Cache-Control"))
		}
		in := frames(t, res.Body)
		if got := next(t, in); got != runUpdated {
			t.Fatalf("connect frame %q", got)
		}
		sigs.send("20261113-a", domain.SignalRunUpdated)
		for got := next(t, in); got != runUpdated; got = next(t, in) {
			if got != ": ping" {
				t.Fatalf("frame %q", got)
			}
		}
		for got := next(t, in); got != ": ping"; got = next(t, in) {
			if got != runUpdated {
				t.Fatalf("frame %q", got)
			}
		}
		if !strings.Contains(logs.String(), "panel stream") || !strings.Contains(logs.String(), "open=1") {
			t.Errorf("open is not logged: %q", logs.String())
		}
	})

	t.Run("the board stream signals runs-changed", func(t *testing.T) {
		srv := eventsServer(t, newFakeSignals(), &syncBuf{}, nil)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()
		res, err := nethttp.Get(ts.URL + "/events")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		if got := next(t, frames(t, res.Body)); got != "event: runs-changed\ndata: runs-changed" {
			t.Fatalf("connect frame %q", got)
		}
	})

	t.Run("an unknown run answers 404", func(t *testing.T) {
		srv := eventsServer(t, newFakeSignals(), &syncBuf{}, nil)
		if rec := get(srv.Handler(), "localhost", "/events?run=missing"); rec.Code != 404 {
			t.Fatalf("status %d", rec.Code)
		}
	})

	t.Run("a closed client frees its subscription and logs the close", func(t *testing.T) {
		logs := &syncBuf{}
		sigs := newFakeSignals("20261113-a")
		srv := eventsServer(t, sigs, logs, nil)
		ts := httptest.NewServer(srv.Handler())
		defer ts.Close()
		res, err := nethttp.Get(ts.URL + "/events?run=20261113-a")
		if err != nil {
			t.Fatal(err)
		}
		next(t, frames(t, res.Body))
		res.Body.Close()
		deadline := time.Now().Add(2 * time.Second)
		for sigs.cancels() == 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if sigs.cancels() != 1 {
			t.Fatalf("cancel called %d times", sigs.cancels())
		}
		if !strings.Contains(logs.String(), "open=0") {
			t.Errorf("close is not logged: %q", logs.String())
		}
	})

	t.Run("Serve ends the streams before Shutdown", func(t *testing.T) {
		sigs := newFakeSignals("20261113-a")
		srv := eventsServer(t, sigs, &syncBuf{}, nil)
		ln, err := Listen(0)
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- srv.Serve(ctx, ln) }()

		res, err := nethttp.Get("http://" + ln.Addr().String() + "/events?run=20261113-a")
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		in := frames(t, res.Body)
		next(t, in)

		start := time.Now()
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("serve returned %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("an open stream held the shutdown")
		}
		if took := time.Since(start); took > 2*time.Second {
			t.Errorf("shutdown took %v", took)
		}
	})

	t.Run("a blocked write ends the stream at its deadline", func(t *testing.T) {
		const timeout = 200 * time.Millisecond
		logs := &syncBuf{}
		sigs := newFakeSignals("20261113-a")
		srv := eventsServer(t, sigs, logs, func(c *Config) { c.WriteTimeout = timeout; c.PingEvery = time.Hour })
		w := &stalledWriter{header: nethttp.Header{}, release: make(chan struct{})}
		defer close(w.release)
		req := httptest.NewRequest(nethttp.MethodGet, "/events?run=20261113-a", nil)
		req.Host = "localhost"
		done := make(chan struct{})
		start := time.Now()
		go func() { srv.Handler().ServeHTTP(w, req); close(done) }()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("a blocked write kept the stream open past its deadline")
		}
		if took := time.Since(start); took < timeout-20*time.Millisecond {
			t.Errorf("the stream ended after %v, before the %v deadline", took, timeout)
		}
		if sigs.cancels() != 1 {
			t.Errorf("cancel called %d times", sigs.cancels())
		}
		w.mu.Lock()
		defer w.mu.Unlock()
		if len(w.deadlines) == 0 {
			t.Fatal("no write deadline was set")
		}
		if d := w.deadlines[0].Sub(start); d <= 0 || d > timeout+time.Second {
			t.Errorf("deadline is %v ahead, want about %v", d, timeout)
		}
	})
}

// stalledWriter blocks every write until the write deadline passes, then fails
// it, as a client that stopped reading does. A write with no deadline set
// blocks until release is closed.
type stalledWriter struct {
	mu        sync.Mutex
	header    nethttp.Header
	deadlines []time.Time
	release   chan struct{}
}

func (w *stalledWriter) Header() nethttp.Header { return w.header }
func (w *stalledWriter) WriteHeader(int)        {}
func (w *stalledWriter) Flush()                 {}

func (w *stalledWriter) Write([]byte) (int, error) {
	w.mu.Lock()
	var deadline time.Time
	if n := len(w.deadlines); n > 0 {
		deadline = w.deadlines[n-1]
	}
	w.mu.Unlock()
	if deadline.IsZero() {
		<-w.release
		return 0, errors.New("write: released with no deadline")
	}
	select {
	case <-time.After(time.Until(deadline)):
		return 0, errors.New("write: i/o timeout")
	case <-w.release:
		return 0, errors.New("write: released before the deadline")
	}
}

func (w *stalledWriter) SetWriteDeadline(t time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.deadlines = append(w.deadlines, t)
	return nil
}

func TestHostGuardEvents(t *testing.T) {
	srv := eventsServer(t, newFakeSignals("20261113-a"), &syncBuf{}, nil)
	h := srv.Handler()
	for _, host := range []string{"", "evil.com", "localhost.evil.com", "127.0.0.1.evil.com", "[::1]:80", "0.0.0.0:80"} {
		for _, target := range []string{"/events", "/events?run=20261113-a"} {
			if rec := get(h, host, target); rec.Code != 403 {
				t.Errorf("host %q %s: status %d", host, target, rec.Code)
			}
		}
	}
	// A loopback host passes the guard: an unknown run answers 404, not 403.
	if rec := get(h, "localhost:80", "/events?run=missing"); rec.Code != 404 {
		t.Errorf("loopback host: status %d", rec.Code)
	}
}

func TestNoPlanPage(t *testing.T) {
	var logs bytes.Buffer
	srv, _ := newServer(t, &logs)
	h := srv.Handler()
	links, err := webkit.Linked()
	if err != nil {
		t.Fatal(err)
	}

	rec := get(h, "localhost", "/runs/noplan")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	for _, want := range []string{
		`<body hx-ext="morph,sse" sse-connect="/events?run=noplan">`,
		`id="summary"`, `hx-get="/runs/noplan/regions/summary"`, `data-v="p2.r2"`,
		`hx-trigger="sse:run-updated, every 5s [!window.harnessLive]"`, `hx-swap="morph:outerHTML"`,
		"no plan &lt;b&gt;", "planning", "noplan",
		links.Htmx, links.Morph, links.Sse, links.Panel, links.Live, links.Icon, `class="live-dot"`,
		string(webkit.Stylesheet()),
	} {
		if !strings.Contains(body, want) {
			t.Errorf("no-plan page lacks %q", want)
		}
	}
	if strings.Contains(body, "http-equiv") || strings.Contains(body, "<b>") {
		t.Errorf("no-plan page keeps the old placeholder or an unescaped title")
	}

	region := "/runs/noplan/regions/summary"
	for _, v := range []string{"p2.r2", "p2.r1"} {
		if rec := get(h, "localhost", region+"?v="+v); rec.Code != 204 || rec.Body.Len() != 0 || rec.Header().Get("HX-Refresh") != "" {
			t.Errorf("v=%s: status %d refresh %q", v, rec.Code, rec.Header().Get("HX-Refresh"))
		}
	}
	// The plan appeared: the plan part differs, so the page reloads.
	rec = get(h, "localhost", region+"?v=p0.r2")
	if rec.Code != 200 || rec.Header().Get("HX-Refresh") != "true" {
		t.Errorf("plan appeared: status %d refresh %q", rec.Code, rec.Header().Get("HX-Refresh"))
	}
}

func TestTrailingSlashRedirects(t *testing.T) {
	var logs bytes.Buffer
	srv, _ := newServer(t, &logs)
	rec := get(srv.Handler(), "localhost", "/runs/20261113-a/")
	if rec.Code != 301 || rec.Header().Get("Location") != "/runs/20261113-a" {
		t.Fatalf("status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
	if rec := get(srv.Handler(), "evil.com", "/runs/20261113-a/"); rec.Code != 403 {
		t.Errorf("guard: status %d", rec.Code)
	}
}

func TestFavicon(t *testing.T) {
	var logs bytes.Buffer
	srv, _ := newServer(t, &logs)
	h := srv.Handler()
	links, err := webkit.Linked()
	if err != nil {
		t.Fatal(err)
	}
	rec := get(h, "localhost", "/favicon.ico")
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "image/svg+xml" || !strings.Contains(rec.Body.String(), "<svg") {
		t.Fatalf("status %d type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	icon := get(h, "localhost", links.Icon)
	if icon.Code != 200 || icon.Body.String() != rec.Body.String() {
		t.Errorf("icon asset differs from the favicon")
	}
	for _, target := range []string{"/", "/runs/noplan"} {
		if body := get(h, "localhost", target).Body.String(); !strings.Contains(body, `<link rel="icon" href="`+links.Icon+`" type="image/svg+xml">`) {
			t.Errorf("%s lacks the icon link", target)
		}
	}
}

func TestRegionMissingLogged(t *testing.T) {
	var logs bytes.Buffer
	srv, _ := newServer(t, &logs)
	if rec := get(srv.Handler(), "localhost", "/runs/20261113-a/regions/nope"); rec.Code != 404 {
		t.Fatalf("status %d", rec.Code)
	}
	out := logs.String()
	if !strings.Contains(out, "panel region missing") || !strings.Contains(out, "run=20261113-a") || !strings.Contains(out, "region=nope") {
		t.Errorf("log %q", out)
	}
}

func TestBoardLive(t *testing.T) {
	var logs bytes.Buffer
	runs := fakeRuns{cards: []domain.Card{
		{ID: "20261113-133205-r3-panel", Title: "R3 panel live\nsecond line", Phase: "planning", Version: domain.Version{Plan: "p1", Run: "r1"}},
	}}
	srv := New(Config{
		Board:  board.NewHandler(runs),
		Page:   page.NewHandler(runs, &fakePages{}),
		Logger: slog.New(slog.NewTextHandler(&logs, nil)),
		Now: func() time.Time {
			at, _ := time.ParseInLocation("20060102-150405", "20261113-134405", time.Local)
			return at
		},
	})
	body := get(srv.Handler(), "localhost", "/").Body.String()
	links, _ := webkit.Linked()
	for _, want := range []string{
		`<body hx-ext="morph,sse" sse-connect="/events">`,
		`hx-trigger="sse:runs-changed, every 5s [!window.harnessLive]"`,
		`<span class="live-dot"`, links.Live, links.Sse, links.Icon,
		`<span class="rid" title="20261113-133205-r3-panel">20261113-133205</span>`,
		`<h3><a href="/runs/20261113-133205-r3-panel" title="R3 panel live
second line">R3 panel live</a></h3>`,
		`<span class="age">12m</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("board lacks %q", want)
		}
	}
}
