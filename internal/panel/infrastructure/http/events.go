package http

import (
	"errors"
	"fmt"
	nethttp "net/http"
	"time"

	"github.com/90p-labs/harness/internal/panel/domain"
)

const (
	defaultPingEvery    = 15 * time.Second
	defaultWriteTimeout = 10 * time.Second
)

// events streams the change signals of the board, or of one run. It sends one
// signal on connect, one for each change and a ping comment on a timer. A
// failed write, a closed client or a stopping server ends it.
func (s *Server) events(w nethttp.ResponseWriter, r *nethttp.Request) {
	if s.cfg.Signals == nil {
		nethttp.NotFound(w, r)
		return
	}
	// An empty run is the board stream, and it takes no repo. A run needs the
	// repo that holds it.
	var ref domain.RunRef
	if run := r.URL.Query().Get("run"); run != "" {
		repo := r.URL.Query().Get("repo")
		if _, err := domain.ParseRepoKey(repo); err != nil {
			nethttp.NotFound(w, r)
			return
		}
		ref = domain.RunRef{Repo: repo, ID: run}
	}
	run, repo := ref.ID, ref.Repo
	ch, cancel, err := s.cfg.Signals.Subscribe(r.Context(), ref)
	if errors.Is(err, domain.ErrNotFound) {
		nethttp.NotFound(w, r)
		return
	}
	if err != nil {
		s.fail(w, "event stream", err)
		return
	}
	defer cancel()
	s.logger.Info("panel stream", "event", "open", "open", s.open.Add(1), "repo", repo, "run", run)
	defer func() {
		s.logger.Info("panel stream", "event", "close", "open", s.open.Add(-1), "repo", repo, "run", run)
	}()

	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	rc := nethttp.NewResponseController(w)
	send := func(frame string) bool {
		// A server with no write timeout may not support the deadline: the
		// write below still reports a failure.
		if err := rc.SetWriteDeadline(time.Now().Add(s.writeTimeout)); err != nil && !errors.Is(err, nethttp.ErrNotSupported) {
			return false
		}
		if _, err := fmt.Fprint(w, frame); err != nil {
			return false
		}
		if err := rc.Flush(); err != nil && !errors.Is(err, nethttp.ErrNotSupported) {
			return false
		}
		return true
	}

	name := domain.SignalRunUpdated
	if run == "" {
		name = domain.SignalRunsChanged
	}
	if !send(signalFrame(name)) {
		return
	}
	ping := time.NewTicker(s.pingEvery)
	defer ping.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.stopping:
			return
		case sig, ok := <-ch:
			if !ok || !send(signalFrame(sig)) {
				return
			}
		case <-ping.C:
			if !send(": ping\n\n") {
				return
			}
		}
	}
}

func signalFrame(name string) string {
	return "event: " + name + "\ndata: " + name + "\n\n"
}
