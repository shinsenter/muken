// Copyright 2026 Mai Nhut Tan
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"crypto/subtle"
	_ "embed"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed index.html
var indexHTML []byte

type Options struct {
	Addr      string
	Token     string
	Interval  time.Duration
	Collector *Collector
	Webhook   *Webhook
	Logf      func(format string, a ...any)
}

type Server struct {
	options Options
	alerter *Alerter
	mu      sync.RWMutex
	snap    Snapshot
}

// New checks the options. Without a token the server must listen on loopback.
func New(options Options) (*Server, error) {
	if options.Interval == 0 {
		options.Interval = 15 * time.Second
	}
	if options.Logf == nil {
		options.Logf = func(string, ...any) {}
	}
	if options.Token == "" {
		host, _, err := net.SplitHostPort(options.Addr)
		if err != nil || (host != "127.0.0.1" && host != "localhost" && host != "::1") {
			return nil, errors.New("a token is required when the agent listens on a non-loopback address")
		}
	}
	return &Server{options: options, alerter: NewAlerter(2)}, nil
}

func (s *Server) refresh(ctx context.Context) {
	snap := s.options.Collector.Collect(ctx)
	for _, msg := range s.alerter.Evaluate(snap) {
		s.options.Logf("alert: %s", msg)
		if err := s.options.Webhook.Send(ctx, msg); err != nil {
			s.options.Logf("webhook failed: %v", err)
		}
	}
	snap.Alerts = s.alerter.Active()

	s.mu.Lock()
	s.snap = snap
	s.mu.Unlock()
}

func (s *Server) current() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snap
}

// authorized accepts the token as a Bearer header or as the Basic auth password.
func (s *Server) authorized(r *http.Request) bool {
	if s.options.Token == "" {
		return true
	}
	got := ""
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		got = strings.TrimPrefix(header, "Bearer ")
	} else if _, password, ok := r.BasicAuth(); ok {
		got = password
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.options.Token)) == 1
}

func (s *Server) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorized(r) {
			w.Header().Set("WWW-Authenticate", `Basic realm="muken"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// Handler returns the HTTP routes. Only /healthz is open without the token.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/api/status", s.guard(s.handleStatus))
	mux.HandleFunc("/metrics", s.guard(s.handleMetrics))
	mux.HandleFunc("/", s.guard(handleIndex))
	return mux
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.current())
}

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	_, _ = w.Write([]byte(Metrics(s.current())))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'unsafe-inline'; script-src 'unsafe-inline'")
	_, _ = w.Write(indexHTML)
}

func (s *Server) Run(ctx context.Context) error {
	s.refresh(ctx)
	go s.refreshEvery(ctx)

	httpServer := &http.Server{Addr: s.options.Addr, Handler: s.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go shutdownOnDone(ctx, httpServer)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) refreshEvery(ctx context.Context) {
	ticker := time.NewTicker(s.options.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.refresh(ctx)
		}
	}
}

func shutdownOnDone(ctx context.Context, httpServer *http.Server) {
	<-ctx.Done()
	grace, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(grace)
}
