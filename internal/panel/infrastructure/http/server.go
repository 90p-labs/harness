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

	"github.com/90p-labs/harness/internal/panel/application/query/board"
	"github.com/90p-labs/harness/internal/panel/application/query/page"
	"github.com/90p-labs/harness/internal/panel/domain"
	"github.com/90p-labs/harness/internal/shared/infrastructure/webkit"
)

//go:embed board.html.tmpl
var boardFiles embed.FS

//go:embed board.css
var boardCSS string

//go:embed board.js
var boardJS string

var templates = template.Must(template.ParseFS(boardFiles, "board.html.tmpl"))

const (
	htmlType       = "text/html; charset=utf-8"
	assetCache     = "public, max-age=31536000, immutable"
	placeholderDoc = `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta http-equiv="refresh" content="5">` +
		`<title>harness panel</title></head><body><p>This run has no plan yet.</p></body></html>`
)

// Config holds what the server needs: the two queries and the identity it reports.
type Config struct {
	Version string
	Repo    string
	Board   board.Handler
	Page    page.Handler
	Logger  *slog.Logger
}

// Server is the panel http adapter.
type Server struct {
	cfg    Config
	logger *slog.Logger
}

// New builds the server. A nil logger falls back to the default one.
func New(cfg Config) *Server {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{cfg: cfg, logger: logger}
}

// Handler routes the panel, behind the Host guard.
func (s *Server) Handler() nethttp.Handler {
	mux := nethttp.NewServeMux()
	mux.HandleFunc("GET /{$}", s.boardPage)
	mux.HandleFunc("GET /regions/board", s.boardRegion)
	mux.HandleFunc("GET /runs/{id}", s.runPage)
	mux.HandleFunc("GET /runs/{id}/regions/{name}", s.runRegion)
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
}

func (s *Server) renderBoard(w nethttp.ResponseWriter, name string, board domain.Board) {
	links, err := webkit.Linked()
	if err != nil {
		s.fail(w, "link board assets", err)
		return
	}
	var buf bytes.Buffer
	view := boardView{Board: board, Links: links}
	if name == "page" {
		view.Kit, view.CSS, view.JS = webkit.Stylesheet(), template.CSS(boardCSS), template.JS(boardJS)
	}
	if err := templates.ExecuteTemplate(&buf, name, view); err != nil {
		s.fail(w, "render board", err)
		return
	}
	writeHTML(w, buf.Bytes())
}

func (s *Server) boardPage(w nethttp.ResponseWriter, r *nethttp.Request) {
	res, err := s.cfg.Board.Handle(r.Context(), board.Query{})
	if err != nil {
		s.fail(w, "board", err)
		return
	}
	s.renderBoard(w, "page", res.Board)
}

func (s *Server) boardRegion(w nethttp.ResponseWriter, r *nethttp.Request) {
	res, err := s.cfg.Board.Handle(r.Context(), board.Query{Version: r.URL.Query().Get("v")})
	if err != nil {
		s.fail(w, "board region", err)
		return
	}
	if res.Unchanged {
		w.WriteHeader(nethttp.StatusNoContent)
		return
	}
	s.renderBoard(w, "region", res.Board)
}

func (s *Server) runPage(w nethttp.ResponseWriter, r *nethttp.Request) {
	res, err := s.cfg.Page.Handle(r.Context(), page.Query{RunID: r.PathValue("id")})
	switch {
	case errors.Is(err, domain.ErrNoPlan), err == nil && res.NoPlan:
		writeHTML(w, []byte(placeholderDoc))
	case errors.Is(err, domain.ErrNotFound):
		nethttp.NotFound(w, r)
	case err != nil:
		s.fail(w, "run page", err)
	default:
		writeHTML(w, res.Body)
	}
}

func (s *Server) runRegion(w nethttp.ResponseWriter, r *nethttp.Request) {
	res, err := s.cfg.Page.Handle(r.Context(), page.Query{
		RunID:   r.PathValue("id"),
		Region:  r.PathValue("name"),
		Version: r.URL.Query().Get("v"),
	})
	switch {
	case errors.Is(err, domain.ErrNotFound), errors.Is(err, domain.ErrNoPlan):
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
	_ = json.NewEncoder(w).Encode(map[string]string{"version": s.cfg.Version, "repo": s.cfg.Repo})
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
