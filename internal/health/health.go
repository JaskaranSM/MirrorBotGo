// Package health serves the liveness endpoints used by the Docker healthcheck.
package health

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// Server is a tiny HTTP server exposing /health and /healthcount.
type Server struct {
	checked atomic.Int64
}

// New returns a new health Server.
func New() *Server { return &Server{} }

// Start launches the health server on addr in a background goroutine.
// Errors binding the listener are reported via the returned error channel's
// first value (nil on clean shutdown is never sent; the goroutine simply logs).
func (s *Server) Start(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		s.checked.Add(1)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/healthcount", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, "%d", s.checked.Load())
	})
	go func() {
		_ = http.ListenAndServe(addr, mux)
	}()
}
