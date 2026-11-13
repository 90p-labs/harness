package http

import (
	"context"
	"errors"
	"fmt"
	"net"
	nethttp "net/http"
	"strconv"
	"time"
)

const (
	readHeaderTimeout = 10 * time.Second
	shutdownTimeout   = 5 * time.Second
)

// Listen binds 127.0.0.1 only. Port 0 picks a free port.
func Listen(port int) (net.Listener, error) {
	ln, err := net.Listen("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return nil, fmt.Errorf("listen on loopback port %d: %w", port, err)
	}
	return ln, nil
}

func (s *Server) httpServer() *nethttp.Server {
	return &nethttp.Server{Handler: s.Handler(), ReadHeaderTimeout: readHeaderTimeout}
}

// Serve answers on the listener until the context ends, then drains requests.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := s.httpServer()
	failed := make(chan error, 1)
	go func() { failed <- srv.Serve(ln) }()
	select {
	case err := <-failed:
		return err
	case <-ctx.Done():
	}
	stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(stop); err != nil {
		// The drain hit its timeout: cut the open connections.
		return errors.Join(fmt.Errorf("stop the panel server: %w", err), srv.Close())
	}
	if err := <-failed; err != nil && !errors.Is(err, nethttp.ErrServerClosed) {
		return err
	}
	return nil
}
