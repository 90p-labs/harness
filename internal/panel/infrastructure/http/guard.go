package http

import (
	"log/slog"
	"net"
	nethttp "net/http"
	"strings"
)

// hostRefusal says why a Host header is not loopback, or "" when it is.
func hostRefusal(host string) string {
	if host == "" {
		return "empty host"
	}
	// SplitHostPort strips brackets, and no IPv6 literal is allowed, so any bracket is refused.
	if strings.ContainsAny(host, "[]") {
		return "malformed host"
	}
	name := host
	if strings.Contains(host, ":") {
		var port string
		var err error
		name, port, err = net.SplitHostPort(host)
		if err != nil || port == "" || !digits(port) {
			return "malformed host"
		}
	}
	switch strings.ToLower(name) {
	case "127.0.0.1", "localhost":
		return ""
	}
	return "host is not loopback"
}

func digits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// guardHost answers 403 to any Host that is not 127.0.0.1 or localhost, which
// stops a rebound DNS name from reading the repo state.
func guardHost(logger *slog.Logger, next nethttp.Handler) nethttp.Handler {
	return nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if reason := hostRefusal(r.Host); reason != "" {
			logger.Warn("panel host refused", "reason", reason, "host", r.Host, "path", r.URL.Path)
			nethttp.Error(w, "forbidden", nethttp.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
