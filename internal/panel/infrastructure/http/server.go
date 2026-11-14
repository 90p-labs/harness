// Package http serves the panel board and run pages on loopback.
package http

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"html/template"
	"log/slog"
	nethttp "net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/90p-labs/harness/internal/panel/application/query/board"
	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/webkit"
)

//go:embed board.html.tmpl noplan.html.tmpl
var boardFiles embed.FS

//go:embed board.css
var boardCSS string

//go:embed board.js
var boardJS string

var templates = template.Must(template.ParseFS(boardFiles, "board.html.tmpl", "noplan.html.tmpl"))

const (
	htmlType   = "text/html; charset=utf-8"
	svgType    = "image/svg+xml"
	assetCache = "public, max-age=31536000, immutable"
)

// Config holds what the server needs: the two queries, the signal source of
// the event stream, the clock of the card age and the version it reports.
type Config struct {
	Version string
	Board   board.Handler
	Page    page.Handler
	Signals domain.Signals
	// Now gives the time of a card age. Nil means time.Now.
	Now    func() time.Time
	Logger *slog.Logger
	// PingEvery is the gap between ping comments of a stream. Zero means 15s.
	PingEvery time.Duration
	// WriteTimeout is the deadline of each stream write. Zero means 10s.
	WriteTimeout time.Duration
}

// Server is the panel http adapter.
type Server struct {
	cfg          Config
	logger       *slog.Logger
	now          func() time.Time
	pingEvery    time.Duration
	writeTimeout time.Duration

	open     atomic.Int64
	stopping chan struct{}
	stopOnce sync.Once
}

// New builds the server. A nil logger falls back to the default one.
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{cfg: cfg, logger: logger, now: cfg.Now, pingEvery: cfg.PingEvery, writeTimeout: cfg.WriteTimeout,
		stopping: make(chan struct{})}
	if s.now == nil {
		s.now = time.Now
	}
	if s.pingEvery <= 0 {
		s.pingEvery = defaultPingEvery
	}
	if s.writeTimeout <= 0 {
		s.writeTimeout = defaultWriteTimeout
	}
	return s
}

// stopStreams ends every open event stream. Serve calls it before Shutdown.
func (s *Server) stopStreams() {
	s.stopOnce.Do(func() { close(s.stopping) })
}

// Handler routes the panel, behind the Host guard.
func (s *Server) Handler() nethttp.Handler {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("GET /{$}", s.boardPage)
	mux.HandleFunc("GET /regions/board", s.boardRegion)
	mux.HandleFunc("GET /repos/{key}/runs/{id}", s.runPage)
	mux.HandleFunc("GET /repos/{key}/runs/{id}/{$}", s.trailingSlash)
	mux.HandleFunc("GET /repos/{key}/runs/{id}/regions/{name}", s.runRegion)
	mux.HandleFunc("GET /runs/{id}", s.legacyRun)
	mux.HandleFunc("GET /runs/{id}/{$}", s.legacyRun)
	mux.HandleFunc("GET /events", s.events)
	mux.HandleFunc("GET /favicon.ico", s.favicon)
	mux.HandleFunc("GET /assets/{hash}/{file}", s.asset)
	mux.HandleFunc("GET /healthz", s.healthz)
	return guardHost(s.logger, mux)
}

type boardView struct {
	Board domain.Board
	Links webkit.Links
	Kit   template.CSS
	CSS   template.CSS
	JS    template.JS
	Now   time.Time
}

// Age gives the age of a card at the server clock.
func (v boardView) Age(c domain.Card) string { return c.Age(v.Now) }

func (s *Server) renderBoard(w nethttp.ResponseWriter, name string, board domain.Board) {
	links, err := webkit.Linked()
	if err != nil {
		s.fail(w, "link board assets", err)
		return
	}
	var buf bytes.Buffer
	view := boardView{Board: board, Links: links, Now: s.now()}
	if name == "page" {
		view.Kit, view.CSS, view.JS = webkit.Stylesheet(), template.CSS(boardCSS), template.JS(boardJS)
	}
	if err := templates.ExecuteTemplate(&buf, name, view); err != nil {
		s.fail(w, "render board", err)
		return
	}
	writeHTML(w, buf.Bytes())
}

// boardFilter reads the repo filter of the query. An absent or empty value
// means no filter. Any other value must be a repo key: a malformed one answers
// 404 and ends the request, before any store call.
func boardFilter(w nethttp.ResponseWriter, r *nethttp.Request) (string, bool) {
	value := r.URL.Query().Get("repo")
	if value == "" {
		return "", true
	}
	key, err := domain.ParseRepoKey(value)
	if err != nil {
		nethttp.NotFound(w, r)
		return "", false
	}
	return key, true
}

func (s *Server) boardPage(w nethttp.ResponseWriter, r *nethttp.Request) {
	repo, ok := boardFilter(w, r)
	if !ok {
		return
	}
	res, err := s.cfg.Board.Handle(r.Context(), board.Query{Repo: repo})
	if err != nil {
		s.fail(w, "board", err)
		return
	}
	if res.UnknownRepo {
		nethttp.Redirect(w, r, "/", nethttp.StatusSeeOther)
		return
	}
	s.renderBoard(w, "page", res.Board)
}

func (s *Server) boardRegion(w nethttp.ResponseWriter, r *nethttp.Request) {
	repo, ok := boardFilter(w, r)
	if !ok {
		return
	}
	res, err := s.cfg.Board.Handle(r.Context(), board.Query{Repo: repo, Version: r.URL.Query().Get("v")})
	if err != nil {
		s.fail(w, "board region", err)
		return
	}
	if res.UnknownRepo {
		w.Header().Set("HX-Redirect", "/")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(nethttp.StatusOK)
		return
	}
	if res.Unchanged {
		w.WriteHeader(nethttp.StatusNoContent)
		return
	}
	s.renderBoard(w, "region", res.Board)
}

// repoKey reads the repo key of the route. A bad key answers 404 and ends the
// request, as an unknown repo does.
func repoKey(w nethttp.ResponseWriter, r *nethttp.Request) (string, bool) {
	key, err := domain.ParseRepoKey(r.PathValue("key"))
	if err != nil {
		nethttp.NotFound(w, r)
		return "", false
	}
	return key, true
}

// legacyRun sends /runs/{id} to the repo route when exactly one repo holds the
// id. Any other count is a 404.
func (s *Server) legacyRun(w nethttp.ResponseWriter, r *nethttp.Request) {
	ref, err := s.cfg.Page.Resolve(r.Context(), r.PathValue("id"))
	switch {
	case errors.Is(err, domain.ErrNotFound):
		nethttp.NotFound(w, r)
	case err != nil:
		s.fail(w, "legacy run", err)
	default:
		nethttp.Redirect(w, r, "/repos/"+ref.Repo+"/runs/"+url.PathEscape(ref.ID), nethttp.StatusMovedPermanently)
	}
}

func (s *Server) runPage(w nethttp.ResponseWriter, r *nethttp.Request) {
	key, ok := repoKey(w, r)
	if !ok {
		return
	}
	res, err := s.cfg.Page.Handle(r.Context(), page.Query{Repo: key, RunID: r.PathValue("id")})
	switch {
	case errors.Is(err, domain.ErrNotFound):
		nethttp.NotFound(w, r)
	case err != nil:
		s.fail(w, "run page", err)
	case res.NoPlan:
		s.renderNoPlan(w, res)
	default:
		writeHTML(w, res.Body)
	}
}

type noPlanView struct {
	Card    domain.Card
	Version string
	Links   webkit.Links
	Kit     template.CSS
	Events  string
	Base    string
}

func (s *Server) renderNoPlan(w nethttp.ResponseWriter, res page.Result) {
	links, err := webkit.Linked()
	if err != nil {
		s.fail(w, "link no-plan assets", err)
		return
	}
	var buf bytes.Buffer
	view := noPlanView{
		Card: res.Card, Version: res.Version, Links: links, Kit: webkit.Stylesheet(),
		Events: res.Events,
		Base:   res.Base,
	}
	if err := templates.ExecuteTemplate(&buf, "noplan", view); err != nil {
		s.fail(w, "render no-plan page", err)
		return
	}
	writeHTML(w, buf.Bytes())
}

// trailingSlash sends /repos/{key}/runs/{id}/ to the same route with no slash.
func (s *Server) trailingSlash(w nethttp.ResponseWriter, r *nethttp.Request) {
	key, ok := repoKey(w, r)
	if !ok {
		return
	}
	nethttp.Redirect(w, r, "/repos/"+key+"/runs/"+url.PathEscape(r.PathValue("id")), nethttp.StatusMovedPermanently)
}

func (s *Server) favicon(w nethttp.ResponseWriter, r *nethttp.Request) {
	links, err := webkit.Linked()
	if err != nil {
		s.fail(w, "link favicon", err)
		return
	}
	body, _, ok := webkit.Asset(links.Hash, "icon.svg")
	if !ok {
		nethttp.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", svgType)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

func (s *Server) runRegion(w nethttp.ResponseWriter, r *nethttp.Request) {
	key, ok := repoKey(w, r)
	if !ok {
		return
	}
	res, err := s.cfg.Page.Handle(r.Context(), page.Query{
		Repo:    key,
		RunID:   r.PathValue("id"),
		Region:  r.PathValue("name"),
		Version: r.URL.Query().Get("v"),
	})
	switch {
	case errors.Is(err, domain.ErrRegionNotFound):
		// Only a known run with an unknown region name is the missing-region
		// signal. An unknown or deleted run is a plain 404.
		s.logger.Warn("panel region missing", "repo", key, "run", r.PathValue("id"), "region", r.PathValue("name"))
		nethttp.NotFound(w, r)
	case errors.Is(err, domain.ErrNotFound):
		nethttp.NotFound(w, r)
	case err != nil:
		s.fail(w, "run region", err)
	case res.Unchanged:
		w.WriteHeader(nethttp.StatusNoContent)
	case res.Refresh:
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(nethttp.StatusOK)
	default:
		writeHTML(w, res.Body)
	}
}

func (s *Server) asset(w nethttp.ResponseWriter, r *nethttp.Request) {
	body, contentType, ok := webkit.Asset(r.PathValue("hash"), r.PathValue("file"))
	if !ok {
		nethttp.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", assetCache)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

func (s *Server) healthz(w nethttp.ResponseWriter, _ *nethttp.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{"version": s.cfg.Version, "pid": os.Getpid()})
}

func writeHTML(w nethttp.ResponseWriter, body []byte) {
	w.Header().Set("Content-Type", htmlType)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(body)
}

// fail logs a server error and answers 500 without its detail.
func (s *Server) fail(w nethttp.ResponseWriter, reason string, err error) {
	s.logger.Error("panel request failed", "reason", reason, "error", err)
	nethttp.Error(w, "internal error", nethttp.StatusInternalServerError)
}
